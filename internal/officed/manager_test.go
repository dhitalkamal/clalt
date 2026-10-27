package officed

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fake world: what is alive, what answers the handshake, what was started, signalled and opened
type world struct {
	t        *testing.T
	m        Manager
	out      *bytes.Buffer
	alive    map[int]bool
	verified map[int]bool
	signals  []int
	spawned  int
	opened   []string
	// onSpawn mimics the child writing its run file
	onSpawn func(w *world, pid int)
	// diesOnStop makes a signalled pid disappear
	diesOnStop bool
	exited     chan struct{}
}

func newWorld(t *testing.T) *world {
	w := &world{t: t, out: &bytes.Buffer{}, alive: map[int]bool{}, verified: map[int]bool{}, exited: make(chan struct{})}
	w.diesOnStop = true
	w.m = Manager{
		Dir: filepath.Join(t.TempDir(), "office"), GOOS: "darwin", Version: "0.2.0", Out: w.out,
		Now: func() time.Time { return time.Date(2026, 10, 28, 11, 30, 0, 0, time.UTC) },
		Verify: func(r Run) error {
			if w.verified[r.PID] {
				return nil
			}
			return errors.New("no answer")
		},
		Alive: func(pid int) bool { return w.alive[pid] },
		Stop: func(pid int) error {
			w.signals = append(w.signals, pid)
			if w.diesOnStop {
				w.alive[pid] = false
			}
			return nil
		},
		Spawn: func(log string) (Child, error) {
			w.spawned++
			pid := 900 + w.spawned
			w.alive[pid], w.verified[pid] = true, true
			if w.onSpawn != nil {
				w.onSpawn(w, pid)
			}
			return Child{PID: pid, Exited: w.exited}, nil
		},
		Open:  func(u string) error { w.opened = append(w.opened, u); return nil },
		Sleep: func(time.Duration) {},
	}
	w.onSpawn = func(w *world, pid int) {
		WriteRun(w.m.Dir, Run{PID: pid, Port: 47321, Token: "secret", StartedAt: w.m.Now(), Version: "0.2.0"})
	}
	return w
}

func (w *world) seed(pid int, live bool) {
	WriteRun(w.m.Dir, Run{PID: pid, Port: 47321, Token: "secret", StartedAt: w.m.Now().Add(-90 * time.Minute), Version: "0.2.0"})
	w.alive[pid], w.verified[pid] = live, live
}

func TestOnStartsOnceAndOpensTheLink(t *testing.T) {
	w := newWorld(t)
	if err := w.m.On(); err != nil {
		t.Fatal(err)
	}
	if w.spawned != 1 || len(w.opened) != 1 || w.opened[0] != "http://127.0.0.1:47321/?t=secret" {
		t.Errorf("spawned %d opened %v", w.spawned, w.opened)
	}
	if strings.Contains(w.out.String(), "secret") {
		t.Errorf("token printed: %s", w.out)
	}
}

func TestOnWhenRunningOnlyOpens(t *testing.T) {
	w := newWorld(t)
	w.seed(555, true)
	if err := w.m.On(); err != nil {
		t.Fatal(err)
	}
	if w.spawned != 0 || len(w.opened) != 1 {
		t.Errorf("spawned %d opened %v", w.spawned, w.opened)
	}
	if !strings.Contains(w.out.String(), "already running") {
		t.Errorf("out %q", w.out)
	}
}

func TestOnReplacesAStaleRunFile(t *testing.T) {
	w := newWorld(t)
	w.seed(555, false)
	if err := w.m.On(); err != nil {
		t.Fatal(err)
	}
	if w.spawned != 1 || len(w.signals) != 0 {
		t.Errorf("spawned %d signals %v", w.spawned, w.signals)
	}
}

func TestOnReportsAChildThatDies(t *testing.T) {
	w := newWorld(t)
	w.onSpawn = func(w *world, pid int) { close(w.exited) }
	os.MkdirAll(w.m.Dir, 0o700)
	os.WriteFile(filepath.Join(w.m.Dir, "office.log"), []byte("t listen failed: address in use\n"), 0o600)
	err := w.m.On()
	if err == nil || !strings.Contains(err.Error(), "address in use") || len(w.opened) != 0 {
		t.Errorf("err %v opened %v", err, w.opened)
	}
}

func TestOnRefusesWindows(t *testing.T) {
	w := newWorld(t)
	w.m.GOOS = "windows"
	err := w.m.On()
	if err == nil || !strings.Contains(err.Error(), "Windows") || w.spawned != 0 {
		t.Errorf("err %v", err)
	}
}

func TestOffStopsAVerifiedServer(t *testing.T) {
	w := newWorld(t)
	w.seed(555, true)
	if err := w.m.Off(); err != nil {
		t.Fatal(err)
	}
	if len(w.signals) != 1 || w.signals[0] != 555 {
		t.Errorf("signals %v", w.signals)
	}
	if _, err := ReadRun(w.m.Dir); !errors.Is(err, ErrNoRun) {
		t.Error("run file left")
	}
}

func TestOffNeverSignalsAnUnverifiedPid(t *testing.T) {
	w := newWorld(t)
	// the pid is alive but is not a clalt office server
	w.seed(555, false)
	w.alive[555] = true
	if err := w.m.Off(); err != nil {
		t.Fatal(err)
	}
	if len(w.signals) != 0 {
		t.Fatalf("signalled %v", w.signals)
	}
	if _, err := ReadRun(w.m.Dir); !errors.Is(err, ErrNoRun) {
		t.Error("stale file left")
	}
	if !strings.Contains(w.out.String(), "stale") {
		t.Errorf("out %q", w.out)
	}
}

func TestOffWithoutARunFile(t *testing.T) {
	w := newWorld(t)
	if err := w.m.Off(); err != nil || len(w.signals) != 0 || !strings.Contains(w.out.String(), "not running") {
		t.Errorf("%v %q", err, w.out)
	}
}

func TestOffGivesUpAfterFiveSecondsWithoutKilling(t *testing.T) {
	w := newWorld(t)
	w.seed(555, true)
	w.diesOnStop = false
	err := w.m.Off()
	if err == nil || len(w.signals) != 1 {
		t.Errorf("err %v signals %v", err, w.signals)
	}
	if _, err := ReadRun(w.m.Dir); err != nil {
		t.Error("run file removed while the server still lives")
	}
}

func TestOpenNeedsARunningServer(t *testing.T) {
	w := newWorld(t)
	err := w.m.OpenLink()
	if err == nil || !strings.Contains(err.Error(), "clalt office on") || len(w.opened) != 0 {
		t.Errorf("err %v", err)
	}
	w.seed(555, true)
	if err := w.m.OpenLink(); err != nil || len(w.opened) != 1 {
		t.Errorf("err %v opened %v", err, w.opened)
	}
}

func TestStatusOnWithAndWithoutLink(t *testing.T) {
	w := newWorld(t)
	w.seed(555, true)
	os.WriteFile(filepath.Join(w.m.Dir, "office.log"), []byte("x review failed: boom\ny ok\n"), 0o600)
	w.m.Status(false)
	got := w.out.String()
	for _, want := range []string{"on", "47321", "555", "1h30m", "0.2.0", "review failed: boom", "--link"} {
		if !strings.Contains(got, want) {
			t.Errorf("status lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "secret") {
		t.Errorf("token without --link:\n%s", got)
	}
	w.out.Reset()
	w.m.Status(true)
	if !strings.Contains(w.out.String(), "http://127.0.0.1:47321/?t=secret") {
		t.Errorf("no link:\n%s", w.out)
	}
}

func TestStatusOffAndStale(t *testing.T) {
	w := newWorld(t)
	w.m.Status(true)
	if !strings.Contains(w.out.String(), "off") {
		t.Errorf("off: %q", w.out)
	}
	w.out.Reset()
	w.seed(555, false)
	w.m.Status(true)
	if !strings.Contains(w.out.String(), "off") || strings.Contains(w.out.String(), "secret") {
		t.Errorf("stale: %q", w.out)
	}
}
