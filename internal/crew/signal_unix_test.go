//go:build !windows

package crew

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestTerminateRefusesAPidThatWouldSignalMoreThanOneProcess(t *testing.T) {
	// the shell of a developer is in the group that kill(0) reaches, so if this
	// ever signalled, the test run itself would end
	for _, pid := range []int{0, 1, -1, -5, -4242} {
		if err := terminate(pid); err == nil {
			t.Errorf("terminate(%d) was allowed", pid)
		}
	}
}

func TestTerminateAsksOneProcessToStop(t *testing.T) {
	pid, _ := startScript(t, "sleep 30", Spec{})
	if err := terminate(pid); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the process to end", func() bool { return syscall.Kill(pid, 0) == syscall.ESRCH })
}

func TestTerminateReachesTheChildrenOfAGatewayThatLeadsItsSession(t *testing.T) {
	childFile := filepath.Join(t.TempDir(), "child.pid")
	pid, _ := startScript(t, "sleep 60 &\necho $! > "+childFile+"\nwait", Spec{})
	var child int
	waitFor(t, "the child", func() bool {
		n, err := strconv.Atoi(strings.TrimSpace(readFile(childFile)))
		child = n
		return err == nil && n > 1
	})
	t.Cleanup(func() { _ = syscall.Kill(child, syscall.SIGKILL) })
	if err := terminate(pid); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the gateway to end", func() bool { return syscall.Kill(pid, 0) == syscall.ESRCH })
	waitFor(t, "its child to end", func() bool { return syscall.Kill(child, 0) == syscall.ESRCH })
}

func TestTerminateOnlySignalsOneProcessWhenItDoesNotLeadAGroup(t *testing.T) {
	// the test process leads the group of the shell it runs in, not its own
	// group: this one must not reach anything beyond the child
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	if err := terminate(cmd.Process.Pid); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Error("sleep ended normally")
	}
}
