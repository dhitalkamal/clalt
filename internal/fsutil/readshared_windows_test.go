//go:build windows

package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

// a reader holding the file open must not stop the writer from replacing it,
// and that has to hold without the retry, which only waits a reader out
func TestAnOpenReaderDoesNotBlockTheReplace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := openShared(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	old := retryRename
	retryRename = false
	t.Cleanup(func() { retryRename = old })

	if err := WriteFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("replace with a reader open: %v", err)
	}
	if got, err := ReadFile(path); err != nil || string(got) != "new" {
		t.Errorf("read %q, %v", got, err)
	}
}
