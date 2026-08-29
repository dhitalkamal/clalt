// Package livestore reads and writes Claude Code's own active credential, which
// lives in two synced places on macOS: the Keychain entry "Claude Code-credentials"
// and ~/.claude/.credentials.json. clalt keeps both in sync when it swaps accounts.
package livestore

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/dhitalkamal/clalt/internal/credential"
	"github.com/dhitalkamal/clalt/internal/keychain"
)

// LiveService is the Keychain service name Claude Code uses.
const LiveService = "Claude Code-credentials"

// wrapper is the on-disk / in-keychain JSON shape: {"claudeAiOauth": {...}}.
type wrapper struct {
	ClaudeAiOauth credential.Credential `json:"claudeAiOauth"`
}

// Store swaps credentials in and out of Claude Code's live locations. On macOS
// the credential lives in both the Keychain and the file; off macOS (kc is nil)
// Claude Code uses the file alone, so this store is file-only there.
type Store struct {
	kc       keychain.Store // nil off macOS: the file is the sole store
	credPath string
}

// New builds a Store over the given keychain and .credentials.json path. Pass a
// nil keychain on platforms where Claude Code stores its credential in the file
// only (Linux, Windows); the store then reads and writes just the file.
func New(kc keychain.Store, credPath string) *Store {
	return &Store{kc: kc, credPath: credPath}
}

// SyncReport describes whether the file and keychain agree.
type SyncReport struct {
	InSync      bool
	FilePresent bool
	KeyPresent  bool
	FileToken   string // access token tail-safe: full value stays internal
	KeyToken    string
}

// Read returns the current live credential. When the file and keychain disagree
// (the known desync bug), it returns whichever has the later expiry so a switch
// write-back never captures a stale token.
func (s *Store) Read() (credential.Credential, error) {
	fileC, fileOK := s.readFile()
	keyC, keyOK := s.readKeychain()
	switch {
	case fileOK && keyOK:
		if keyC.ExpiresAt > fileC.ExpiresAt {
			return keyC, nil
		}
		return fileC, nil
	case fileOK:
		return fileC, nil
	case keyOK:
		return keyC, nil
	default:
		return credential.Credential{}, errors.New("no live Claude Code credential found in file or keychain")
	}
}

// Write persists cred to BOTH the file and the keychain entry.
func (s *Store) Write(c credential.Credential) error {
	raw, err := json.Marshal(wrapper{ClaudeAiOauth: c})
	if err != nil {
		return err
	}
	if err := s.writeFile(raw); err != nil {
		return err
	}
	if s.kc == nil {
		return nil // file-only platforms: no keychain mirror
	}
	return s.kc.Set(LiveService, s.liveAccount(), raw)
}

// SyncStatus compares the file and keychain copies for the doctor command.
func (s *Store) SyncStatus() (SyncReport, error) {
	fileC, fileOK := s.readFile()
	keyC, keyOK := s.readKeychain()
	rep := SyncReport{FilePresent: fileOK, KeyPresent: keyOK}
	if fileOK {
		rep.FileToken = fileC.AccessToken
	}
	if keyOK {
		rep.KeyToken = keyC.AccessToken
	}
	if s.kc == nil {
		// file-only platforms: the file is authoritative, there is no keychain
		// to disagree with, so a present file is by definition in sync.
		rep.InSync = fileOK
		return rep, nil
	}
	rep.InSync = fileOK && keyOK && fileC.AccessToken == keyC.AccessToken
	return rep, nil
}

// liveAccount reuses the existing keychain account name if one exists so we
// overwrite the exact entry Claude Code reads; otherwise it falls back to $USER.
func (s *Store) liveAccount() string {
	if s.kc != nil {
		if names, err := s.kc.List(LiveService); err == nil && len(names) > 0 {
			return names[0]
		}
	}
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "clalt"
}

func (s *Store) readFile() (credential.Credential, bool) {
	raw, err := os.ReadFile(s.credPath)
	if err != nil {
		return credential.Credential{}, false
	}
	var w wrapper
	if err := json.Unmarshal(bytes.TrimSpace(raw), &w); err != nil {
		return credential.Credential{}, false
	}
	if w.ClaudeAiOauth.AccessToken == "" {
		return credential.Credential{}, false
	}
	return w.ClaudeAiOauth, true
}

func (s *Store) readKeychain() (credential.Credential, bool) {
	if s.kc == nil {
		return credential.Credential{}, false // file-only platforms
	}
	names, err := s.kc.List(LiveService)
	if err != nil || len(names) == 0 {
		return credential.Credential{}, false
	}
	raw, err := s.kc.Get(LiveService, names[0])
	if err != nil {
		return credential.Credential{}, false
	}
	var w wrapper
	if err := json.Unmarshal(raw, &w); err != nil {
		return credential.Credential{}, false
	}
	if w.ClaudeAiOauth.AccessToken == "" {
		return credential.Credential{}, false
	}
	return w.ClaudeAiOauth, true
}

func (s *Store) writeFile(raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(s.credPath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.credPath, raw, 0o600)
}
