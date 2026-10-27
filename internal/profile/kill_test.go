//go:build !windows

package profile

import (
	"os/exec"
	"testing"
	"time"
)

func TestKillEndsProcessAndPrunesRecord(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper process: %v", err)
	}
	pid := cmd.Process.Pid
	if err := WriteSession(dir, Session{Account: "work", PID: pid, StartedAt: time.Now()}); err != nil {
		t.Fatalf("write session: %v", err)
	}

	if err := Kill(dir, pid); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	// the session record must be gone
	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("record not pruned: %v", sessions)
	}
	// reap and confirm the process actually terminated
	_ = cmd.Wait()
	if alive(pid) {
		t.Error("process still alive after Kill")
	}
}

func TestKillMissingPidIsGraceful(t *testing.T) {
	dir := t.TempDir()
	// killing a pid with no record and no process should not hang or panic
	_ = Kill(dir, 0)
}
