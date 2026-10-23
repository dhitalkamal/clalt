//go:build !windows

package crew

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// script writes an executable sh script into a temp dir and returns its path.
func script(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prog")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOutputReturnsWhatTheProcessPrintedOnEachStream(t *testing.T) {
	stdout, stderr, err := ExecRunner{}.Output(context.Background(), Spec{Bin: script(t, "echo out; echo err >&2")})
	if err != nil || string(stdout) != "out\n" || string(stderr) != "err\n" {
		t.Errorf("got %q %q %v", stdout, stderr, err)
	}
}

func TestAFailingProcessKeepsItsOutputAndReportsTheExit(t *testing.T) {
	stdout, stderr, err := ExecRunner{}.Output(context.Background(), Spec{Bin: script(t, "echo partial; echo bad >&2; exit 3")})
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("err = %v, want exit 3", err)
	}
	if string(stdout) != "partial\n" || string(stderr) != "bad\n" {
		t.Errorf("got %q %q", stdout, stderr)
	}
}

func TestAProgramThatIsNotThereIsAnErrorThatIsNotAnExit(t *testing.T) {
	_, _, err := ExecRunner{}.Output(context.Background(), Spec{Bin: filepath.Join(t.TempDir(), "missing")})
	var exit *exec.ExitError
	if err == nil || errors.As(err, &exit) {
		t.Errorf("err = %v", err)
	}
}

func TestOutputUsesTheEnvironmentItIsGivenAndNothingElse(t *testing.T) {
	t.Setenv("CLALT_TEST_AMBIENT", "leaked")
	bin := script(t, `echo "${FOO}|${CLALT_TEST_AMBIENT}"`)
	stdout, _, err := ExecRunner{}.Output(context.Background(), Spec{Bin: bin, Env: []string{"FOO=bar"}})
	if err != nil || string(stdout) != "bar|\n" {
		t.Errorf("got %q, %v", stdout, err)
	}
	stdout, _, err = ExecRunner{}.Output(context.Background(), Spec{Bin: bin})
	if err != nil || string(stdout) != "|leaked\n" {
		t.Errorf("with no environment given, got %q, %v", stdout, err)
	}
}

func TestOutputRunsInTheFolderItIsGiven(t *testing.T) {
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	stdout, _, err := ExecRunner{}.Output(context.Background(), Spec{Bin: script(t, "pwd -P"), Dir: dir})
	if err != nil || strings.TrimSpace(string(stdout)) != dir {
		t.Errorf("got %q, %v, want %s", stdout, err, dir)
	}
}

func TestOutputStopsWhenTheContextEnds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	began := time.Now()
	_, _, err := ExecRunner{}.Output(ctx, Spec{Bin: script(t, "sleep 30")})
	if err == nil || time.Since(began) > 10*time.Second {
		t.Errorf("err = %v after %v", err, time.Since(began))
	}
}

// startScript starts body detached with its output in a log, and kills it when
// the test ends.
func startScript(t *testing.T, body string, spec Spec) (pid int, log string) {
	t.Helper()
	spec.Bin = script(t, body)
	log = filepath.Join(t.TempDir(), "gateway.out")
	pid, err := ExecRunner{}.Start(spec, log)
	if err != nil {
		t.Fatal(err)
	}
	// kill with a pid of 0 or below would signal a whole group or every process
	if pid <= 1 {
		t.Fatalf("Start returned the pid %d", pid)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	return pid, log
}

// waitFor polls until the condition holds, or fails the test after five seconds.
func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestStartLaunchesTheProcessInASessionOfItsOwn(t *testing.T) {
	pid, log := startScript(t, "echo started; echo oops >&2; sleep 30", Spec{})
	waitFor(t, "the output", func() bool { return strings.Contains(readFile(log), "oops") })
	if got := readFile(log); !strings.Contains(got, "started") {
		t.Errorf("stdout is not in the log: %q", got)
	}
	if sid, err := unix.Getsid(pid); err != nil || sid != pid {
		t.Errorf("session of %d is %d (%v), want its own", pid, sid, err)
	}
	if err := syscall.Kill(pid, 0); err != nil {
		t.Errorf("the process is not running after Start returned: %v", err)
	}
}

func TestStartAppendsToTheLogAndKeepsItPrivate(t *testing.T) {
	spec := Spec{Bin: script(t, "echo new")}
	log := filepath.Join(t.TempDir(), "gateway.out")
	if err := os.WriteFile(log, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (ExecRunner{}).Start(spec, log); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the output", func() bool { return strings.Contains(readFile(log), "new") })
	if got := readFile(log); got != "old\nnew\n" {
		t.Errorf("log = %q", got)
	}
	fresh := filepath.Join(t.TempDir(), "fresh.out")
	if _, err := (ExecRunner{}).Start(spec, fresh); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the output", func() bool { return readFile(fresh) != "" })
	if fi, err := os.Stat(fresh); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("a new log has mode %v, %v, want 0600", fi.Mode().Perm(), err)
	}
}

func TestStartGivesTheProcessItsEnvironmentAndFolder(t *testing.T) {
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	_, log := startScript(t, `echo "$FOO|$(pwd -P)"`, Spec{Env: []string{"FOO=bar"}, Dir: dir})
	waitFor(t, "the output", func() bool { return readFile(log) != "" })
	if got := strings.TrimSpace(readFile(log)); got != "bar|"+dir {
		t.Errorf("log = %q", got)
	}
}

func TestAStartedProcessHasNoInput(t *testing.T) {
	_, log := startScript(t, `if read x; then echo input; else echo eof; fi`, Spec{})
	waitFor(t, "the output", func() bool { return readFile(log) != "" })
	if got := strings.TrimSpace(readFile(log)); got != "eof" {
		t.Errorf("log = %q, want eof", got)
	}
}

func TestStartOfAProgramThatIsNotThereIsAnError(t *testing.T) {
	log := filepath.Join(t.TempDir(), "gateway.out")
	if pid, err := (ExecRunner{}).Start(Spec{Bin: filepath.Join(t.TempDir(), "missing")}, log); err == nil {
		t.Errorf("Start gave pid %d", pid)
	}
}

func TestAStartedProcessIsNotLeftAZombieWhenItEnds(t *testing.T) {
	pid, _ := startScript(t, "exit 0", Spec{})
	// a zombie still answers signal 0, so gone means it was reaped
	waitFor(t, "the process to be gone", func() bool { return syscall.Kill(pid, 0) == syscall.ESRCH })
}

func TestProbeSeesARealProcessThroughPs(t *testing.T) {
	if _, err := exec.LookPath("ps"); err != nil {
		t.Skip("no ps")
	}
	pid, _ := startScript(t, "sleep 30", Spec{})
	p, err := Probe(context.Background(), ExecRunner{}, os.Environ(), pid)
	if err != nil || !p.Alive || p.Start == "" || !strings.Contains(p.Command, "prog") {
		t.Fatalf("Probe = %+v, %v", p, err)
	}
	again, _ := Probe(context.Background(), ExecRunner{}, os.Environ(), pid)
	if again.Start != p.Start {
		t.Errorf("the start time changed between two looks: %q and %q", p.Start, again.Start)
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "ps to lose the process", func() bool {
		q, err := Probe(context.Background(), ExecRunner{}, os.Environ(), pid)
		return err == nil && !q.Alive
	})
}
