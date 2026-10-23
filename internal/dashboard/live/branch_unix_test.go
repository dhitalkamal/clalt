//go:build !windows

package live

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestBranchDoesNotWaitOnAPipeWhereHeadShouldBe(t *testing.T) {
	root := t.TempDir()
	git := filepath.Join(root, ".git")
	if err := os.MkdirAll(git, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(git, "HEAD"), 0o600); err != nil {
		t.Skip("no fifo here")
	}
	done := make(chan string, 1)
	go func() { done <- gitBranch(root) }()
	select {
	case got := <-done:
		if got != "" {
			t.Errorf("branch = %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("gitBranch is stuck on a pipe")
	}
}
