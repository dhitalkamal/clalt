package crew

import (
	"context"
	"sync"
)

// fakeRunner stands in for the operating system: it answers every Output with
// what answer says and remembers every call. nothing is ever started.
type fakeRunner struct {
	mu     sync.Mutex
	calls  []Spec
	starts []Spec
	logs   []string
	answer func(Spec) (stdout, stderr string, err error)
	pid    int   // what Start returns
	failed error // what Start fails with
}

func (f *fakeRunner) Output(_ context.Context, s Spec) ([]byte, []byte, error) {
	f.mu.Lock()
	f.calls = append(f.calls, s)
	f.mu.Unlock()
	out, errOut, err := f.answer(s)
	return []byte(out), []byte(errOut), err
}

func (f *fakeRunner) Start(s Spec, logPath string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts = append(f.starts, s)
	f.logs = append(f.logs, logPath)
	return f.pid, f.failed
}

// exitErr is what a program that ended with a non-zero code looks like.
type exitErr struct{ code int }

func (e exitErr) Error() string { return "exit status" }

func (e exitErr) ExitCode() int { return e.code }
