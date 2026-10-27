package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/runcmd"
	"github.com/dhitalkamal/clalt/internal/terminal"
)

// the tests of this package never run kitty, tmux, osascript or notify-send and
// never show a notification. unless a test installs fakes with useFakeDeps, the
// runner of the real deps refuses to run anything and finds no program.
func TestTheRunnerOfTheRealDepsRefusesInTests(t *testing.T) {
	d := (&App{}).deps()

	// a name no machine has, so that this test is harmless even without the guard
	_, err := d.runner.Run(context.Background(), "clalt-guard-probe", "new-window")

	if err == nil || !strings.Contains(err.Error(), "without installing fake deps") {
		t.Errorf("err = %v, want the refusal", err)
	}
	if d.runner.Has("sh") || d.runner.Has("tmux") || d.runner.Has("osascript") {
		t.Error("the refusing runner finds a program")
	}
}

func TestFakeDepsDoNotSeeTheRealEnvironment(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1/default,1,0")
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/kitty")
	e := newEnv(t)

	e.useFakeDeps()

	d := e.app.deps()
	if d.env("TMUX") != "" || d.env("KITTY_LISTEN_ON") != "" {
		t.Error("the fake environment shows the shell the tests run in")
	}
}

func TestFakeDepsHoldWhatATestPutsIn(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()
	f.env["TMUX"] = "/tmp/tmux-1/default,1,0"
	f.running = []terminal.Session{{Handle: "h1"}}
	f.lastSeen["abc"] = "work"

	d := e.app.deps()
	running, _ := d.sessions()
	id, ok := d.lastAccount("abc")

	if d.env("TMUX") == "" || len(running) != 1 || !ok || id != "work" {
		t.Errorf("env %q, sessions %v, last account %q %v", d.env("TMUX"), running, id, ok)
	}
	if _, ok := d.runner.(*runcmd.Fake); !ok {
		t.Errorf("runner is %T", d.runner)
	}
}

// refusingRunner is what the real deps get in these tests.
type refusingRunner struct{}

func (refusingRunner) Run(_ context.Context, name string, _ ...string) ([]byte, error) {
	return nil, &refusal{name}
}

func (refusingRunner) Has(string) bool { return false }

type refusal struct{ name string }

func (r *refusal) Error() string { return "a test ran " + r.name + " without installing fake deps" }

// productionRunner is the runner the real deps get when no test is running.
var productionRunner = newRunner

func init() {
	newRunner = func() runcmd.Runner { return refusingRunner{} }
}

// fakeDeps is what the session commands see outside clalt in a test: a fake
// runner, an environment that holds only what the test puts in it, a fixed
// executable path, and canned answers from the registry and the feed.
type fakeDeps struct {
	run      *runcmd.Fake
	env      map[string]string
	running  []terminal.Session
	runErr   error
	lastSeen map[string]string // claude session id -> account id
}

// useFakeDeps swaps the real deps of the test's app for fakes.
func (e *testEnv) useFakeDeps() *fakeDeps {
	e.t.Helper()
	f := &fakeDeps{run: runcmd.NewFake(), env: map[string]string{}, lastSeen: map[string]string{}}
	e.app.sdeps = &sessionDeps{
		runner: f.run,
		env:    func(name string) string { return f.env[name] },
		exe:    func() (string, error) { return "/opt/clalt/bin/clalt", nil },
		goos:   "darwin",
		sessions: func() ([]terminal.Session, error) {
			return append([]terminal.Session(nil), f.running...), f.runErr
		},
		lastAccount: func(id string) (string, bool) {
			account, ok := f.lastSeen[id]
			return account, ok
		},
	}
	return f
}
