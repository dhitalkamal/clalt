package claudecli

import (
	"fmt"
	"os"
	"os/exec"
)

// Binary resolves the claude executable: CLALT_CLAUDE_BIN when set, else claude,
// looked up on PATH. resolving first gives one clear error up front instead of
// a failure from deep inside a command.
func Binary() (string, error) {
	name := os.Getenv("CLALT_CLAUDE_BIN")
	if name == "" {
		name = "claude"
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("cannot run %q (install claude, or point CLALT_CLAUDE_BIN at it): %w", name, err)
	}
	return path, nil
}
