//go:build !windows

package wrapper

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// holder starts a shell that notes the signal it gets in a file and exits with the
// code a shell would report for it.
func holder(t *testing.T, trap string) (Process, string) {
	t.Helper()
	marker := filepath.Join(t.TempDir(), "got")
	script := `trap 'echo TERM > "$1"; exit 143' TERM; trap 'echo HUP > "$1"; exit 129' HUP; ` + trap + ` echo ready > "$1.ready"; while :; do sleep 0.02; done`
	cmd := exec.Command("/bin/sh", "-c", script, "sh", marker)
	p, err := StartExec(cmd)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the shell to be ready", func() bool { _, err := os.Stat(marker + ".ready"); return err == nil })
	t.Cleanup(func() { _ = p.Kill(); _ = p.Wait() })
	return p, marker
}

func exitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

func TestTerminateAsksClaudeToEndWithSIGTERM(t *testing.T) {
	p, marker := holder(t, "")

	if err := p.Terminate(); err != nil {
		t.Fatal(err)
	}

	err := p.Wait()
	if code := exitCode(err); code != 143 {
		t.Errorf("exit code = %d, want 143, what claude ends with on SIGTERM", code)
	}
	if !p.EndedByTerminate(err) || p.EndedByTerminate(nil) || p.EndedByTerminate(errors.New("exit status 1")) {
		t.Error("EndedByTerminate does not tell the end SIGTERM makes from a user's exit")
	}
	if got, _ := os.ReadFile(marker); !strings.HasPrefix(string(got), "TERM") {
		t.Errorf("the shell saw %q, want TERM", got)
	}
}

func TestKillEndsAProcessThatIgnoresTerminate(t *testing.T) {
	p, _ := holder(t, "trap '' TERM;")
	_ = p.Terminate()
	time.Sleep(100 * time.Millisecond)
	done := make(chan error, 1)
	go func() { done <- p.Wait() }()
	select {
	case <-done:
		t.Fatal("the process ended on a SIGTERM it ignores")
	default:
	}

	if err := p.Kill(); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		var ee *exec.ExitError
		if !errors.As(err, &ee) || ee.ExitCode() != -1 {
			t.Errorf("Wait = %v, want it killed by a signal", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Kill did not end the process")
	}
}

func TestSignalPassesAnySignalOn(t *testing.T) {
	p, marker := holder(t, "")

	if err := p.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}

	if code := exitCode(p.Wait()); code != 129 {
		t.Errorf("exit code = %d, want 129", code)
	}
	if got, _ := os.ReadFile(marker); !strings.HasPrefix(string(got), "HUP") {
		t.Errorf("the shell saw %q", got)
	}
}

func TestStartExecReportsAStartThatFailsAndPidIsTheChilds(t *testing.T) {
	if _, err := StartExec(exec.Command("/no/such/claude")); err == nil {
		t.Error("a missing binary started")
	}
	p, _ := holder(t, "")
	if p.PID() <= 0 || p.PID() == os.Getpid() {
		t.Errorf("PID = %d", p.PID())
	}
}
