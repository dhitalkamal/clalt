// Package profile manages isolated Claude Code config directories, one per
// account, so accounts can run in parallel. Each profile dir holds its own
// .credentials.json; pointing Claude Code at it via CLAUDE_CONFIG_DIR gives a
// fully isolated session that does not touch the global login.
package profile

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/dhitalkamal/clalt/internal/credential"
)

// wrapper is the {"claudeAiOauth": {...}} shape Claude Code reads.
type wrapper struct {
	ClaudeAiOauth credential.Credential `json:"claudeAiOauth"`
}

// CredPath is the credentials file inside a profile dir.
func CredPath(dir string) string {
	return filepath.Join(dir, ".credentials.json")
}

// HasCredential reports whether a profile dir already holds a credential.
func HasCredential(dir string) bool {
	_, err := os.Stat(CredPath(dir))
	return err == nil
}

// Stage writes cred into the profile dir's credentials file (0600), creating the
// dir if needed. This never touches the global keychain or ~/.claude.
func Stage(dir string, c credential.Credential) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(wrapper{ClaudeAiOauth: c})
	if err != nil {
		return err
	}
	return os.WriteFile(CredPath(dir), raw, 0o600)
}

// Read returns the credential a profile dir currently holds. ok is false when no
// credentials file exists yet (a fresh profile that has not logged in).
func Read(dir string) (c credential.Credential, ok bool, err error) {
	raw, rerr := os.ReadFile(CredPath(dir))
	if rerr != nil {
		if os.IsNotExist(rerr) {
			return credential.Credential{}, false, nil
		}
		return credential.Credential{}, false, rerr
	}
	var w wrapper
	if err := json.Unmarshal(raw, &w); err != nil {
		return credential.Credential{}, false, err
	}
	if w.ClaudeAiOauth.AccessToken == "" {
		return credential.Credential{}, false, nil
	}
	return w.ClaudeAiOauth, true, nil
}
