package claudecli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// writeExe writes a file that passes for an executable and returns its path.
// windows looks a program up by extension, so the file gets .exe there.
func writeExe(t *testing.T, path, body string) string {
	t.Helper()
	path = path + map[bool]string{true: ".exe", false: ""}[runtime.GOOS == "windows"]
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBinaryUsesTheOverride(t *testing.T) {
	bin := writeExe(t, filepath.Join(t.TempDir(), "my-claude"), "not run\n")
	t.Setenv("CLALT_CLAUDE_BIN", bin)
	t.Setenv("PATH", t.TempDir())

	got, err := Binary()

	if err != nil || got != bin {
		t.Errorf("Binary() = %q, %v, want %q", got, err, bin)
	}
}

func TestBinaryLooksNamesUpOnPath(t *testing.T) {
	dir := t.TempDir()
	claude := writeExe(t, filepath.Join(dir, "claude"), "not run\n")
	other := writeExe(t, filepath.Join(dir, "other-claude"), "not run\n")
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
	missing := filepath.Join(t.TempDir(), "no-such-claude")
	t.Setenv("CLALT_CLAUDE_BIN", missing)

	_, err := Binary()

	if err == nil || !strings.Contains(err.Error(), strconv.Quote(missing)) {
		t.Errorf("err = %v, want one that names %s", err, missing)
	}
}
