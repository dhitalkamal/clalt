package config

import (
	"os"
	"path/filepath"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// SharedClaudeDir is the claude home that account folders link into: the user's
// ~/.claude, or CLAUDE_CONFIG_DIR when that points outside clalt. it is empty
// when no absolute path can be worked out (HOME is unset), never a relative one,
// so a caller cannot end up linking into the working directory.
func SharedClaudeDir() string {
	dir, _ := sharedHome()
	return dir
}

// SharedClaudeJSONPath is the global config whose mcpServers are mirrored into
// each account. claude moves it inside the config dir only when
// CLAUDE_CONFIG_DIR is set. it is empty when SharedClaudeDir is.
func SharedClaudeJSONPath() string {
	dir, custom := sharedHome()
	switch {
	case dir == "":
		return ""
	case custom:
		return filepath.Join(dir, ".claude.json")
	}
	return filepath.Join(home(), ".claude.json")
}

// sharedHome skips a CLAUDE_CONFIG_DIR inside clalt's own dir: that is a
// `clalt run` session pointing at an account folder, not the user's home.
func sharedHome() (dir string, custom bool) {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" && !insideClalt(d) {
		abs, err := filepath.Abs(d)
		if err != nil {
			return "", false
		}
		return abs, true
	}
	if h := home(); filepath.IsAbs(h) {
		return filepath.Join(h, ".claude"), false
	}
	return "", false
}

// insideClalt asks the file system, not the spelling: temp dirs on macOS sit
// under /var, a symlink to /private/var, the file system may fold case, and the
// account folder may not exist yet.
func insideClalt(dir string) bool { return fsutil.Within(dir, ClaltDir()) }
