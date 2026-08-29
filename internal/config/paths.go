// Package config resolves filesystem locations for both Claude Code's own store
// and clalt's private state, honoring CLAUDE_CONFIG_DIR and XDG_CONFIG_HOME.
package config

import (
	"os"
	"path/filepath"
)

// claudeConfigDir returns Claude Code's config dir: $CLAUDE_CONFIG_DIR if set,
// else ~/.claude.
func claudeConfigDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	return filepath.Join(home(), ".claude")
}

// ClaudeConfigDir is Claude Code's config dir (the shared home linked into
// profiles by `clalt run --link`).
func ClaudeConfigDir() string { return claudeConfigDir() }

// ClaudeCredentialsPath is where Claude Code keeps .credentials.json.
func ClaudeCredentialsPath() string {
	return filepath.Join(claudeConfigDir(), ".credentials.json")
}

// ClaudeProjectsDir holds the per-session transcript JSONL files.
func ClaudeProjectsDir() string {
	return filepath.Join(claudeConfigDir(), "projects")
}

// ClaudeJSONPath is Claude Code's .claude.json, which holds MCP server
// definitions (top-level mcpServers = user scope, projects.<path>.mcpServers =
// local scope). It sits at ~/.claude.json by default, NOT inside ~/.claude, and
// moves under $CLAUDE_CONFIG_DIR when that is set.
func ClaudeJSONPath() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, ".claude.json")
	}
	return filepath.Join(home(), ".claude.json")
}

// ClaltDir is clalt's own state dir: $XDG_CONFIG_HOME/clalt if set,
// else ~/.config/clalt.
func ClaltDir() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "clalt")
	}
	return filepath.Join(home(), ".config", "clalt")
}

// StatePath is the non-secret metadata + active-pointer file.
func StatePath() string { return filepath.Join(ClaltDir(), "state.json") }

// SwitchLogPath records account-switch events for usage attribution.
func SwitchLogPath() string { return filepath.Join(ClaltDir(), "switches.jsonl") }

// CacheDir holds cached --live usage responses.
func CacheDir() string { return filepath.Join(ClaltDir(), "cache") }

// ProfilesDir holds one isolated Claude Code config dir per account, for parallel
// sessions launched via `clalt run`.
func ProfilesDir() string { return filepath.Join(ClaltDir(), "profiles") }

// ProfileDir is the isolated config dir for one account.
func ProfileDir(name string) string { return filepath.Join(ProfilesDir(), name) }

// SessionsDir holds one record per live `clalt run` session.
func SessionsDir() string { return filepath.Join(ClaltDir(), "sessions") }

// VaultDir holds the encrypted per-account credential store (clalt's own
// secrets, as opposed to Claude Code's live credential, which stays in the
// macOS Keychain).
func VaultDir() string { return filepath.Join(ClaltDir(), "vault") }

// VaultKeyPath is the local encryption key for VaultDir, created on first use.
func VaultKeyPath() string { return filepath.Join(ClaltDir(), "vault.key") }

// KeepaliveAutoMarkerPath records that the background keepalive was auto-enabled,
// so clalt does not re-check or re-announce it on every command.
func KeepaliveAutoMarkerPath() string { return filepath.Join(ClaltDir(), "keepalive.auto") }

// KeepaliveDisabledPath, when present, means the user turned off automatic
// keepalive (via `keepalive uninstall`); clalt then never auto-installs it.
func KeepaliveDisabledPath() string { return filepath.Join(ClaltDir(), "keepalive.disabled") }

// home returns the user home dir, falling back to $HOME so tests can override it.
func home() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}
