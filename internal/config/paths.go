// Package config resolves filesystem locations for both Claude Code's own store
// and clalt's private state, honoring CLAUDE_CONFIG_DIR and XDG_CONFIG_HOME.
package config

import (
	"os"
	"path/filepath"
)

// ClaudeProjectsDir holds the per-session transcript JSONL files: the projects
// folder of the shared claude home. that is the same whichever folder the shell
// points at, since inside a `clalt run` session CLAUDE_CONFIG_DIR is an account
// folder whose projects is only a link. it is empty when the shared home is.
func ClaudeProjectsDir() string {
	shared := SharedClaudeDir()
	if shared == "" {
		return ""
	}
	return filepath.Join(shared, "projects")
}

// ClaltDir is clalt's own state dir: $XDG_CONFIG_HOME/clalt if set,
// else ~/.config/clalt.
func ClaltDir() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "clalt")
	}
	return filepath.Join(home(), ".config", "clalt")
}

// StatePath is the account list: the labels, recorded sign-ins and budgets of
// the accounts. there are no secrets in it.
func StatePath() string { return filepath.Join(ClaltDir(), "state.json") }

// SwitchLogPath is the log of account switches that clalt v0.1.0 kept for usage
// attribution. nothing writes to it now, it is only read.
func SwitchLogPath() string { return filepath.Join(ClaltDir(), "switches.jsonl") }

// ProfilesDir holds the per-account profile folders of clalt v0.1.0. nothing
// writes there now: `clalt migrate` copies their history out and `clalt doctor`
// looks for history that is not copied yet.
func ProfilesDir() string { return filepath.Join(ClaltDir(), "profiles") }

// AccountsDir holds one claude config dir per account, each used as
// CLAUDE_CONFIG_DIR for that account (claude keys the login to the path).
func AccountsDir() string { return filepath.Join(ClaltDir(), "accounts") }

// accountDir is the folder of account id and does not check the id, which is why
// it is not exported: AccountDirFor is the way to get one.
func accountDir(id string) string { return filepath.Join(AccountsDir(), id) }

// SessionsDir holds one record per live `clalt run` session.
func SessionsDir() string { return filepath.Join(ClaltDir(), "sessions") }

// VaultDir held the encrypted per-account token store of clalt v0.1.0. nothing
// reads or writes it now; `clalt doctor` reports it so it can be deleted.
func VaultDir() string { return filepath.Join(ClaltDir(), "vault") }

// VaultKeyPath was the encryption key for VaultDir.
func VaultKeyPath() string { return filepath.Join(ClaltDir(), "vault.key") }

// home returns the user home dir, falling back to $HOME so tests can override it.
func home() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}
