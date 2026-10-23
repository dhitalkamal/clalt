package crew

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const psStart = "Thu Oct 23 15:05:12 2026"

type fakeProc struct {
	start, command string
	alive          bool
}

// world is a fake machine for the manager: a kirocrew that answers --version and
// config set, a ps that knows the processes that were started, and the signals
// that reach them. nothing real is run.
type world struct {
	mu sync.Mutex
	t  *testing.T

	version     string          // what kirocrew --version prints
	configFails map[string]bool // keys whose config set fails
	startErr    error
	diesAtOnce  bool   // a started gateway is gone at the first look
	logText     string // what a started gateway has printed
	ignoresTerm bool   // a stop signal changes nothing
	ready       func(port int) bool
	onPS        func(pid, nth int) // a test can change the world when ps is asked

	procs    map[int]*fakeProc
	nextPID  int
	psAsked  map[int]int
	calls    []Spec // everything run through Output except ps
	started  []Spec
	logs     []string // where each started gateway was told to log
	signals  []int
	psBroken bool
	lsof     string // what lsof prints for a gateway, empty means loopback only
	lsofErr  error
}

func newWorld(t *testing.T) *world {
	t.Helper()
	isolate(t)
	return &world{
		t: t, version: "kirocrew 0.9.0\n", configFails: map[string]bool{}, procs: map[int]*fakeProc{},
		nextPID: 4242, psAsked: map[int]int{}, ready: func(int) bool { return true },
	}
}

func (w *world) Output(_ context.Context, s Spec) ([]byte, []byte, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if s.Bin == "ps" {
		return w.ps(s)
	}
	if s.Bin == "lsof" {
		if w.lsofErr != nil {
			return nil, nil, w.lsofErr
		}
		if w.lsof != "" {
			return []byte(w.lsof), nil, nil
		}
		return []byte("p1\nn127.0.0.1:5500\n"), nil, nil
	}
	w.calls = append(w.calls, s)
	switch {
	case len(s.Args) == 1 && s.Args[0] == "--version":
		return []byte(w.version), nil, nil
	case len(s.Args) == 4 && s.Args[0] == "config" && s.Args[1] == "set":
		if w.configFails[s.Args[2]] {
			return nil, []byte("Unknown key: " + s.Args[2] + "\n"), exitErr{1}
		}
		return []byte("ok\n"), nil, nil
	}
	w.t.Fatalf("the fake kirocrew was run as %v", s.Args)
	return nil, nil, nil
}

func (w *world) ps(s Spec) ([]byte, []byte, error) {
	if w.psBroken {
		return nil, []byte("ps: illegal option\n"), exitErr{1}
	}
	pid, _ := strconv.Atoi(s.Args[len(s.Args)-1])
	w.psAsked[pid]++
	if w.onPS != nil {
		w.mu.Unlock()
		w.onPS(pid, w.psAsked[pid])
		w.mu.Lock()
	}
	p := w.procs[pid]
	if p == nil || !p.alive {
		return nil, nil, exitErr{1}
	}
	return []byte(p.start + " " + p.command + "\n"), nil, nil
}

func (w *world) Start(s Spec, logPath string) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.started = append(w.started, s)
	w.logs = append(w.logs, logPath)
	if w.startErr != nil {
		return 0, w.startErr
	}
	pid := w.nextPID
	w.nextPID++
	w.procs[pid] = &fakeProc{
		start:   psStart,
		command: "/App/python3.12 -s -P -m kiro_crew " + strings.Join(s.Args, " "),
		alive:   !w.diesAtOnce,
	}
	if w.logText != "" {
		if err := os.WriteFile(logPath, []byte(w.logText), 0o600); err != nil {
			w.t.Fatal(err)
		}
	}
	return pid, nil
}

// signal is what a stop sends: it ends the process unless the test says that
// this one ignores it.
func (w *world) signal(pid int) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.signals = append(w.signals, pid)
	if p := w.procs[pid]; p != nil && !w.ignoresTerm {
		p.alive = false
	}
	return nil
}

// plant adds a process that was there before, such as one that an earlier
// clalt started.
func (w *world) plant(pid int, start, command string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.procs[pid] = &fakeProc{start: start, command: command, alive: true}
}

func (w *world) kill(pid int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.procs[pid]
	if p == nil {
		w.t.Fatalf("there is no process %d to end", pid)
	}
	p.alive = false
}

func (w *world) manager() *Manager {
	return NewManager(Options{
		Runner:       w,
		Binary:       func() (string, error) { return "/fake/kirocrew", nil },
		ClaudeBinary: func() (string, error) { return "/fake/claude", nil },
		Environ:      func() []string { return append([]string(nil), ambient...) },
		Now:          func() time.Time { return time.Date(2026, 10, 23, 15, 5, 12, 0, time.UTC) },
		Busy:         func(int) bool { return false },
		Ready:        func(_ context.Context, port int) bool { return w.ready(port) },
		Signal:       w.signal,
		ReadyWait:    200 * time.Millisecond, StopWait: 200 * time.Millisecond, Poll: time.Millisecond,
	})
}

// folder makes an account folder, which the gateway is only given and must
// never write into.
func (w *world) folder(id string) string {
	dir := filepath.Join(w.t.TempDir(), id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		w.t.Fatal(err)
	}
	return dir
}

// recordedPortOf reads the port of an account's record, or fails.
func recordedPortOf(t *testing.T, id string) int {
	t.Helper()
	rec, found, err := ReadRecord(id)
	if err != nil || !found {
		t.Fatalf("no record for %s: %v", id, err)
	}
	return rec.Port
}
