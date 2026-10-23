package crew

import (
	"os"
	"path/filepath"
	"testing"
)

// isolate points HOME and XDG_CONFIG_HOME into a temp dir, so nothing a test
// does can see or touch the real ~/.config/clalt. it returns the home.
func isolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	return home
}

// readFile is the text of a file, or nothing when it cannot be read.
func readFile(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}
