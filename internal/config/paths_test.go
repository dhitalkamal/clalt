package config

import (
	"path/filepath"
	"testing"
)

func TestClaudeCredentialsPathDefault(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("HOME", "/home/tester")
	got := ClaudeCredentialsPath()
	want := "/home/tester/.claude/.credentials.json"
	if got != want {
		t.Errorf("ClaudeCredentialsPath() = %q, want %q", got, want)
	}
}

func TestClaudeCredentialsPathOverride(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/custom/cfg")
	got := ClaudeCredentialsPath()
	want := "/custom/cfg/.credentials.json"
	if got != want {
		t.Errorf("ClaudeCredentialsPath() = %q, want %q", got, want)
	}
}

func TestClaudeProjectsDir(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/custom/cfg")
	got := ClaudeProjectsDir()
	want := "/custom/cfg/projects"
	if got != want {
		t.Errorf("ClaudeProjectsDir() = %q, want %q", got, want)
	}
}

func TestClaltDirRespectsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got := ClaltDir(); got != "/xdg/clalt" {
		t.Errorf("ClaltDir() = %q, want /xdg/clalt", got)
	}
	// derived paths hang off ClaltDir
	if got := StatePath(); got != filepath.Join("/xdg/clalt", "state.json") {
		t.Errorf("StatePath() = %q", got)
	}
	if got := SwitchLogPath(); got != filepath.Join("/xdg/clalt", "switches.jsonl") {
		t.Errorf("SwitchLogPath() = %q", got)
	}
}

func TestClaltDirFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/tester")
	if got := ClaltDir(); got != "/home/tester/.config/clalt" {
		t.Errorf("ClaltDir() = %q, want /home/tester/.config/clalt", got)
	}
}
