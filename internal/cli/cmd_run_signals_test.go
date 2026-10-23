//go:build !windows

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// the test below runs this in a process of its own: clalt run work, against the
// fake that the parent installed. TestMain points every process of this package
// at the guard, so the fake is handed over by name. the helper's stdin is the
// null device, which is no terminal, so that is the case it is in unless the
// parent says it models a terminal.
func TestHelperThatRunsClalt(t *testing.T) {
	if os.Getenv("CLALT_TEST_RUN_HELPER") == "" {
		t.Skip("only runs as the helper of another test")
	}
	t.Setenv("CLALT_CLAUDE_BIN", os.Getenv("CLALT_TEST_FAKE"))
	app := NewApp()
	if os.Getenv("CLALT_TEST_STDIN_TERMINAL") != "" {
		app.terminal = func() bool { return true }
	}
	// the helper shares the group of the test run, which may be in the foreground
	// of the developer's terminal, so the real answer is never used here
	app.foreground = func() bool { return os.Getenv("CLALT_TEST_FOREGROUND") != "" }
	os.Exit(run(app, []string{"run", "work"}))
}

// runningClalt is a clalt run in a process of its own, with claude held in its
// session until the test lets go.
type runningClalt struct {
	t    *testing.T
	e    *testEnv
	cmd  *exec.Cmd
	done chan struct{}
	err  error
	log  string // where clalt's own output goes
}

// startRun starts clalt as it runs at a terminal: the input is a terminal, whose
// Ctrl-C reaches claude by itself.
func startRun(t *testing.T) *runningClalt { return startRunWith(t, true) }

// startHeadlessRun starts clalt with an input that is no terminal, as a
// headless -p run, a script or a supervisor would.
func startHeadlessRun(t *testing.T) *runningClalt { return startRunWith(t, false) }

// startRunWith starts clalt with the input of a terminal or without, and with
// the named signals ignored from the start, the way nohup or a script that puts
// it in the background would.
func startRunWith(t *testing.T, terminal bool, ignored ...string) *runningClalt {
	return startRunMode(t, terminal, false, ignored...)
}

// startForegroundRun starts clalt with an input that is no terminal, in the
// foreground group of a terminal, as `echo hi | clalt run work -p` typed at a
// shell is: the terminal's Ctrl-C reaches claude by itself.
func startForegroundRun(t *testing.T) *runningClalt { return startRunMode(t, false, true) }

func startRunMode(t *testing.T, terminal, foreground bool, ignored ...string) *runningClalt {
	t.Helper()
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.set("SESSION_HOLDS", "1")
	r := &runningClalt{t: t, e: e, done: make(chan struct{})}
	helper := []string{os.Args[0], "-test.run=^TestHelperThatRunsClalt$"}
	if len(ignored) > 0 {
		// an ignored signal stays ignored through exec, which is all nohup does
		helper = append([]string{"/bin/sh", "-c", "trap '' " + strings.Join(ignored, " ") + `; exec "$0" "$@"`}, helper...)
	}
	r.cmd = exec.Command(helper[0], helper[1:]...)
	r.cmd.Env = append(os.Environ(), "CLALT_TEST_RUN_HELPER=1", "CLALT_TEST_FAKE="+os.Getenv("CLALT_CLAUDE_BIN"))
	if terminal {
		r.cmd.Env = append(r.cmd.Env, "CLALT_TEST_STDIN_TERMINAL=1")
	}
	if foreground {
		r.cmd.Env = append(r.cmd.Env, "CLALT_TEST_FOREGROUND=1")
	}
	// a file and not a pipe: claude inherits it, and Wait would not return for a
	// clalt that is gone while a pipe is still held open by claude
	r.log = filepath.Join(t.TempDir(), "clalt.out")
	out, err := os.Create(r.log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { out.Close() })
	r.cmd.Stdout, r.cmd.Stderr = out, out
	if err := r.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { r.err = r.cmd.Wait(); close(r.done) }()
	t.Cleanup(func() {
		r.release()
		_ = r.cmd.Process.Kill()
	})
	r.waitFor("session.holding")
	return r
}

func (r *runningClalt) file(name string) string { return filepath.Join(r.e.claude.dir, name) }

// said is what clalt printed.
func (r *runningClalt) said() string {
	raw, _ := os.ReadFile(r.log)
	return string(raw)
}

func (r *runningClalt) has(name string) bool {
	_, err := os.Stat(r.file(name))
	return err == nil
}

func (r *runningClalt) waitFor(name string) {
	r.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !r.has(name) {
		if time.Now().After(deadline) {
			r.t.Fatalf("gave up waiting for %s. clalt said: %s", name, r.said())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// release lets the session end by itself.
func (r *runningClalt) release() { _ = os.WriteFile(r.file("session.release"), nil, 0o600) }

// ended waits for clalt to end and gives its exit code, or -1 when it was killed
// by a signal.
func (r *runningClalt) ended(within time.Duration) (int, bool) {
	select {
	case <-r.done:
		return r.cmd.ProcessState.ExitCode(), true
	case <-time.After(within):
		return 0, false
	}
}

func (r *runningClalt) signals() string {
	raw, _ := os.ReadFile(r.file("signals"))
	return strings.TrimSpace(string(raw))
}

func (r *runningClalt) claudePID() int {
	r.t.Helper()
	raw, err := os.ReadFile(r.file("session.pid"))
	if err != nil {
		r.t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		r.t.Fatalf("session.pid holds %q", raw)
	}
	return pid
}

// at a terminal a Ctrl-C reaches clalt and claude together, since they share the
// terminal's group, and claude decides what it means. clalt must still be there
// when claude ends, or the shell gets the terminal back while claude is still
// using it. the signal is caught and not ignored, so that claude does not
// inherit an ignore.
func TestAtATerminalAnInterruptDoesNotEndClaltBeforeClaude(t *testing.T) {
	r := startRun(t)

	if err := r.cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}

	if code, ended := r.ended(600 * time.Millisecond); ended {
		t.Fatalf("clalt ended with %d on the interrupt while claude was still running: %s", code, r.said())
	}
	r.release()
	if code, ended := r.ended(10 * time.Second); !ended || code != 0 {
		t.Errorf("after claude ended: ended %v, exit %d: %s", ended, code, r.said())
	}
	// the terminal sends the interrupt to claude itself, so clalt does not
	if got := r.signals(); got != "" {
		t.Errorf("claude was sent %q by clalt", got)
	}
	if left := sessionRecordFiles(r.e); len(left) != 0 {
		t.Errorf("the session record was left behind: %v", left)
	}
}

// the interrupt that the terminal sends to claude is claude's own, and it has to
// get it as the default it expects: a clalt that had ignored the signal would
// hand claude an ignore, through exec, and Ctrl-C would do nothing in claude
func TestAtATerminalClaudeStillGetsItsOwnInterruptAndClaltEndsWithItsCode(t *testing.T) {
	r := startRun(t)
	pid := r.claudePID()

	if err := r.cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	if _, ended := r.ended(300 * time.Millisecond); ended {
		t.Fatalf("clalt ended on the interrupt: %s", r.said())
	}
	if err := syscall.Kill(pid, syscall.SIGINT); err != nil {
		t.Fatal(err)
	}

	code, ended := r.ended(10 * time.Second)
	if got := r.signals(); got != "INT" {
		t.Errorf("claude noted %q, want INT: it did not get its own interrupt. clalt ended: %v, exit %d", got, ended, code)
	}
	if !ended || code != 130 {
		t.Errorf("clalt ended: %v, exit %d, want 130, claude's own: %s", ended, code, r.said())
	}
}

// kill, an ide that closes, a terminal that is closed: the signal is for clalt
// alone, and claude would be left running with nobody to answer to, whatever
// the input is
func TestAStopSignalIsPassedOnToClaudeAndClaltEndsWithItsCode(t *testing.T) {
	for name, c := range map[string]struct {
		sig  syscall.Signal
		code int
	}{"TERM": {syscall.SIGTERM, 143}, "HUP": {syscall.SIGHUP, 129}} {
		for kind, terminal := range map[string]bool{"at a terminal": true, "without one": false} {
			t.Run(name+" "+kind, func(t *testing.T) {
				r := startRunWith(t, terminal)

				if err := r.cmd.Process.Signal(c.sig); err != nil {
					t.Fatal(err)
				}

				code, ended := r.ended(10 * time.Second)
				if got := r.signals(); got != name {
					t.Errorf("claude was sent %q, want %s. clalt ended: %v, exit %d", got, name, ended, code)
				}
				if !ended || code != c.code {
					t.Errorf("clalt ended: %v, exit %d, want %d: %s", ended, code, c.code, r.said())
				}
				if left := sessionRecordFiles(r.e); len(left) != 0 {
					t.Errorf("the session record was left behind: %v", left)
				}
			})
		}
	}
}

// nohup, or a script that puts clalt in the background, starts it with SIGHUP or
// SIGINT ignored, and claude run by hand would inherit that ignore. clalt must
// leave such a signal alone: a caught signal goes back to its default in claude
// through exec, so catching it would take the ignore away. the fake is a shell,
// which cannot trap a signal that was ignored when it started
func TestASignalIgnoredWhenClaltStartsStaysIgnoredForClaude(t *testing.T) {
	for name, sig := range map[string]syscall.Signal{"HUP": syscall.SIGHUP, "INT": syscall.SIGINT} {
		for kind, terminal := range map[string]bool{"at a terminal": true, "without one": false} {
			t.Run(name+" "+kind, func(t *testing.T) { stayIgnored(t, name, sig, terminal) })
		}
	}
}

func stayIgnored(t *testing.T, name string, sig syscall.Signal, terminal bool) {
	t.Helper()
	r := startRunWith(t, terminal, name)

	if err := r.cmd.Process.Signal(sig); err != nil {
		t.Fatal(err)
	}
	if code, ended := r.ended(300 * time.Millisecond); ended {
		t.Fatalf("clalt ended with %d and claude noted %q: %s", code, r.signals(), r.said())
	}
	if err := syscall.Kill(r.claudePID(), sig); err != nil {
		t.Fatal(err)
	}
	if code, ended := r.ended(300 * time.Millisecond); ended {
		t.Fatalf("claude ended on %s, so it was not ignored: claude noted %q, clalt exit %d", name, r.signals(), code)
	}

	r.release()
	if code, ended := r.ended(10 * time.Second); !ended || code != 0 {
		t.Errorf("after claude ended: ended %v, exit %d: %s", ended, code, r.said())
	}
	if got := r.signals(); got != "" {
		t.Errorf("claude noted %q", got)
	}
}
