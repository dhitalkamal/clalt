package claudecli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("these tests use sh scripts as the claude binary")
	}
}

// writeExe writes an executable script and returns its path.
func writeExe(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBinaryUsesTheOverride(t *testing.T) {
	skipOnWindows(t)
	bin := writeExe(t, filepath.Join(t.TempDir(), "my-claude"), "#!/bin/sh\n")
	t.Setenv("CLALT_CLAUDE_BIN", bin)
	t.Setenv("PATH", t.TempDir())

	got, err := Binary()

	if err != nil || got != bin {
		t.Errorf("Binary() = %q, %v, want %q", got, err, bin)
	}
}

func TestBinaryLooksNamesUpOnPath(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	claude := writeExe(t, filepath.Join(dir, "claude"), "#!/bin/sh\n")
	other := writeExe(t, filepath.Join(dir, "other-claude"), "#!/bin/sh\n")
	t.Setenv("PATH", dir)

	t.Setenv("CLALT_CLAUDE_BIN", "")
	if got, err := Binary(); err != nil || got != claude {
		t.Errorf("default: Binary() = %q, %v, want %q", got, err, claude)
	}
	t.Setenv("CLALT_CLAUDE_BIN", "other-claude")
	if got, err := Binary(); err != nil || got != other {
		t.Errorf("override by name: Binary() = %q, %v, want %q", got, err, other)
	}
}

func TestBinaryMissingIsAClearError(t *testing.T) {
	skipOnWindows(t)
	t.Setenv("CLALT_CLAUDE_BIN", "")
	t.Setenv("PATH", t.TempDir())

	_, err := Binary()

	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("err = %v, want it to wrap exec.ErrNotFound", err)
	}
	for _, want := range []string{"claude", "CLALT_CLAUDE_BIN"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestBinaryOverrideThatDoesNotExistIsAnError(t *testing.T) {
	skipOnWindows(t)
	missing := filepath.Join(t.TempDir(), "no-such-claude")
	t.Setenv("CLALT_CLAUDE_BIN", missing)

	_, err := Binary()

	if err == nil || !strings.Contains(err.Error(), missing) {
		t.Errorf("err = %v, want one that names %s", err, missing)
	}
}
