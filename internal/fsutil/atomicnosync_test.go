package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteFileAtomicNoSyncReplacesTheFileAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.json")
	for _, body := range []string{"one", "two"} {
		if err := WriteFileAtomicNoSync(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != body {
			t.Fatalf("read %q, %v; want %q", got, err, body)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("left files behind: %v", entries)
	}
	// windows has no unix permission bits to set
	if fi, _ := os.Stat(path); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", fi.Mode().Perm())
	}
}

func TestWriteFileAtomicNoSyncFailsInAMissingFolderWithoutTemp(t *testing.T) {
	err := WriteFileAtomicNoSync(filepath.Join(t.TempDir(), "nope", "a"), []byte("x"), 0o600)
	if err == nil || !strings.Contains(err.Error(), "temp file") {
		t.Errorf("err = %v", err)
	}
}
