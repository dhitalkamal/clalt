package cli

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/officed"
)

func itoa(n int) string { return strconv.Itoa(n) }

// daemonFakes is a manager and an autostart whose every effect is recorded.
type daemonFakes struct {
	spawned, stops []int
	opened         []string
	launchctl      [][]string
	alive          bool
	verified       bool
}

func fakeDaemon(t *testing.T, e *testEnv) *daemonFakes {
	t.Helper()
	f := &daemonFakes{}
	dir := config.OfficeDir()
	e.app.officeMgr = &officed.Manager{
		Dir: dir, GOOS: "darwin", Version: "0.2.0", Now: func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC) },
		Verify: func(officed.Run) error {
			if f.verified {
				return nil
			}
			return errors.New("no")
		},
		Alive: func(int) bool { return f.alive },
		Stop:  func(pid int) error { f.stops = append(f.stops, pid); f.alive = false; return nil },
		Spawn: func(string) (officed.Child, error) {
			f.spawned = append(f.spawned, 1)
			f.alive, f.verified = true, true
			officed.WriteRun(dir, officed.Run{PID: 31, Port: 47321, Token: "s3cr3t0123", StartedAt: time.Now(), Version: "0.2.0"})
			return officed.Child{PID: 31, Exited: make(chan struct{})}, nil
		},
		Open:  func(u string) error { f.opened = append(f.opened, u); return nil },
		Sleep: func(time.Duration) {},
	}
	e.app.officeAuto = &officed.Autostart{
		Home: e.home, UID: 501, Exe: "/opt/bin/clalt", LogPath: officed.LogPath(dir), GOOS: "darwin",
		Run: func(args []string) ([]byte, error) { f.launchctl = append(f.launchctl, args); return nil, nil },
	}
	return f
}

func TestOfficeOnOpenStatusOff(t *testing.T) {
	e := newEnv(t)
	f := fakeDaemon(t, e)
	if code := e.run("office", "open"); code != 1 || !strings.Contains(e.errOut.String(), "clalt office on") {
		t.Errorf("open while off: %d %q", code, e.errOut)
	}
	e.reset()
	if code := e.run("office", "on"); code != 0 || len(f.spawned) != 1 || len(f.opened) != 1 {
		t.Fatalf("on: %d %q spawned %v opened %v", code, e.errOut, f.spawned, f.opened)
	}
	e.run("office", "on")
	if len(f.spawned) != 1 || len(f.opened) != 2 {
		t.Errorf("second on started again: %v %v", f.spawned, f.opened)
	}
	e.reset()
	e.run("office", "status")
	if strings.Contains(e.out.String(), "s3cr3t0123") || !strings.Contains(e.out.String(), "47321") {
		t.Errorf("status:\n%s", e.out)
	}
	e.reset()
	e.run("office", "status", "--link")
	if !strings.Contains(e.out.String(), "?t=s3cr3t0123") {
		t.Errorf("status --link:\n%s", e.out)
	}
	e.reset()
	if code := e.run("office", "off"); code != 0 || len(f.stops) != 1 || f.stops[0] != 31 {
		t.Errorf("off: %d stops %v", code, f.stops)
	}
}

func TestOfficeDaemonCommandsRefuseExtraArguments(t *testing.T) {
	e := newEnv(t)
	fakeDaemon(t, e)
	for _, args := range [][]string{{"on", "x"}, {"off", "--now"}, {"open", "y"}, {"status", "--links"}, {"autostart"}, {"autostart", "maybe"}, {"autostart", "on", "x"}} {
		e.reset()
		if code := e.run(append([]string{"office"}, args...)...); code != 2 {
			t.Errorf("%v = %d, want 2", args, code)
		}
	}
}

func TestOfficeAutostartOnOffStatus(t *testing.T) {
	e := newEnv(t)
	f := fakeDaemon(t, e)
	plist := filepath.Join(e.home, "Library", "LaunchAgents", "com.dhitalkamal.clalt.office.plist")
	if code := e.run("office", "autostart", "on"); code != 0 {
		t.Fatalf("on: %d %q", code, e.errOut)
	}
	raw, err := os.ReadFile(plist)
	if err != nil || !strings.Contains(string(raw), "/opt/bin/clalt") {
		t.Fatalf("plist %v %s", err, raw)
	}
	if got := strings.Join(f.launchctl[len(f.launchctl)-1], " "); got != "bootstrap gui/501 "+plist {
		t.Errorf("launchctl %q", got)
	}
	e.reset()
	e.run("office", "autostart", "status")
	if !strings.Contains(e.out.String(), "on") {
		t.Errorf("status %q", e.out)
	}
	if code := e.run("office", "autostart", "off"); code != 0 {
		t.Fatalf("off: %d %q", code, e.errOut)
	}
	if _, err := os.Stat(plist); err == nil {
		t.Error("plist left")
	}
	if got := f.launchctl[len(f.launchctl)-1][0]; got != "bootout" {
		t.Errorf("last launchctl %q", got)
	}
}

func TestOfficeAutostartOnLinuxSaysMacOnly(t *testing.T) {
	e := newEnv(t)
	f := fakeDaemon(t, e)
	e.app.officeAuto.GOOS = "linux"
	if code := e.run("office", "autostart", "on"); code != 1 || !strings.Contains(e.errOut.String(), "macOS only") || len(f.launchctl) != 0 {
		t.Errorf("%d %q %v", code, e.errOut, f.launchctl)
	}
}

func TestForegroundOfficeRefusesTheRunningPort(t *testing.T) {
	e := newEnv(t)
	f := fakeDaemon(t, e)
	e.run("office", "on")
	f.opened = nil
	e.reset()
	if code := e.run("office", "--port", "47321", "--no-open"); code != 1 || !strings.Contains(e.errOut.String(), "clalt office off") {
		t.Errorf("%d %q", code, e.errOut)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestServeWritesTheRunFileKeepsTheTokenOutOfTheOutputAndCleansUp(t *testing.T) {
	e := newEnv(t)
	fakeDaemon(t, e)
	out := &lockedOut{}
	e.app.Out = out
	ctx, cancel := context.WithCancel(context.Background())
	e.app.officeCtx = ctx
	port := freePort(t)
	done := make(chan int, 1)
	go func() { done <- e.run("office", "serve", "--port", itoa(port)) }()
	var r officed.Run
	var err error
	for i := 0; i < 300; i++ {
		if r, err = officed.ReadRun(config.OfficeDir()); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil || r.Port != port || r.PID != os.Getpid() || len(r.Token) != 32 {
		t.Fatalf("run file %+v %v", r, err)
	}
	// windows has no unix permission bits to set
	if fi, _ := os.Stat(filepath.Join(config.OfficeDir(), "run.json")); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", fi.Mode())
	}
	if strings.Contains(out.String(), r.Token) {
		t.Errorf("token in the output: %q", out)
	}
	code, _ := fetch(t, "http://127.0.0.1:"+itoa(port)+"/api/ping", r.Token)
	if code != 200 {
		t.Errorf("ping %d", code)
	}
	cancel()
	if c := <-done; c != 0 {
		t.Errorf("exit %d", c)
	}
	if _, err := officed.ReadRun(config.OfficeDir()); err == nil {
		t.Error("run file left after a clean stop")
	}
}

func TestServeWhenAlreadyRunningDoesNotStartASecondServer(t *testing.T) {
	e := newEnv(t)
	f := fakeDaemon(t, e)
	f.alive, f.verified = true, true
	officed.WriteRun(config.OfficeDir(), officed.Run{PID: 31, Port: 47321, Token: "s3cr3t0123", StartedAt: time.Now()})
	if code := e.run("office", "serve", "--port", itoa(freePort(t))); code != 0 {
		t.Errorf("exit %d %q", code, e.errOut)
	}
	if r, _ := officed.ReadRun(config.OfficeDir()); r.PID != 31 {
		t.Errorf("the running server's file was replaced: %+v", r)
	}
}
