package fsutil

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// failRead makes the first failures reads fail with a sharing-style error.
func failRead(t *testing.T, failures int) *int {
	t.Helper()
	calls := 0
	oldRead, oldRetry := openRead, retryRename
	openRead = func(p string) ([]byte, error) {
		calls++
		if calls <= failures {
			return nil, &os.PathError{Op: "open", Path: p, Err: syscall.EACCES}
		}
		return os.ReadFile(p)
	}
	retryRename = true
	t.Cleanup(func() { openRead, retryRename = oldRead, oldRetry })
	return &calls
}

func TestReadFileWaitsOutABusyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := failRead(t, 3)

	got, err := ReadFile(path)
	if err != nil || string(got) != "hi" {
		t.Fatalf("ReadFile = %q, %v", got, err)
	}
	if *calls != 4 {
		t.Errorf("read was tried %d times, want 4", *calls)
	}
}

func TestReadFileGivesUpAfterBoundedRetries(t *testing.T) {
	calls := failRead(t, 1000)
	if _, err := ReadFile(filepath.Join(t.TempDir(), "x")); err == nil {
		t.Fatal("expected an error")
	}
	if *calls != renameAttempts {
		t.Errorf("read was tried %d times, want %d", *calls, renameAttempts)
	}
}

func TestReadFileDoesNotRetryAMissingFile(t *testing.T) {
	calls := 0
	old := openRead
	openRead = func(p string) ([]byte, error) { calls++; return old(p) }
	t.Cleanup(func() { openRead = old })

	_, err := ReadFile(filepath.Join(t.TempDir(), "missing"))
	if !os.IsNotExist(err) || calls != 1 {
		t.Errorf("err %v after %d reads, want not-exist after 1", err, calls)
	}
}
