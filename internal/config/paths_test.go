package config

import (
	"path/filepath"
	"testing"
)

func TestClaudeProjectsDir(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "cfg")
	t.Setenv("CLAUDE_CONFIG_DIR", cfg)
	got := ClaudeProjectsDir()
	want := filepath.Join(cfg, "projects")
	if got != want {
		t.Errorf("ClaudeProjectsDir() = %q, want %q", got, want)
	}
}

// the transcripts are in the shared home, whichever folder the shell points at:
// inside a `clalt run` session CLAUDE_CONFIG_DIR is an account folder, whose
// projects is only a link to them
func TestClaudeProjectsDirIsAlwaysTheSharedHomes(t *testing.T) {
	home, _ := sharedEnv(t)
	shared := filepath.Join(home, ".claude", "projects")

	if got := ClaudeProjectsDir(); got != shared {
		t.Errorf("by default: %q, want %q", got, shared)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", accountDir("robin"))
	if got := ClaudeProjectsDir(); got != shared {
		t.Errorf("inside an account folder: %q, want %q", got, shared)
	}
	custom := filepath.Join(filepath.Dir(home), "elsewhere")
	t.Setenv("CLAUDE_CONFIG_DIR", custom)
	if got, want := ClaudeProjectsDir(), filepath.Join(custom, "projects"); got != want {
		t.Errorf("with a custom home: %q, want %q", got, want)
	}
}

// no home, no folder: never a relative path that would read the working dir
func TestClaudeProjectsDirIsEmptyWithoutAHome(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	if got := ClaudeProjectsDir(); got != "" {
		t.Errorf("ClaudeProjectsDir() = %q, want none", got)
	}
}

func TestClaltDirRespectsXDG(t *testing.T) {
	xdg := t.TempDir()
	clalt := filepath.Join(xdg, "clalt")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if got := ClaltDir(); got != clalt {
		t.Errorf("ClaltDir() = %q, want %q", got, clalt)
	}
	// derived paths hang off ClaltDir
	if got := StatePath(); got != filepath.Join(clalt, "state.json") {
		t.Errorf("StatePath() = %q", got)
	}
	if got := SwitchLogPath(); got != filepath.Join(clalt, "switches.jsonl") {
		t.Errorf("SwitchLogPath() = %q", got)
	}
}

func TestClaltDirFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	h := t.TempDir()
	t.Setenv("HOME", h)
	// the same ~/.config/clalt on every OS: on windows that is under the home folder
	if want := filepath.Join(h, ".config", "clalt"); ClaltDir() != want {
		t.Errorf("ClaltDir() = %q, want %q", ClaltDir(), want)
	}
}

func TestAccountDirsHangOffClaltDir(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if got := AccountsDir(); got != filepath.Join(xdg, "clalt", "accounts") {
		t.Errorf("AccountsDir() = %q", got)
	}
	if got := accountDir("robin-2"); got != filepath.Join(xdg, "clalt", "accounts", "robin-2") {
		t.Errorf("accountDir() = %q", got)
	}
}
