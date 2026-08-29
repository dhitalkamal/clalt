package livestore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/credential"
	"github.com/dhitalkamal/clalt/internal/keychain"
)

func newTestStore(t *testing.T) (*Store, *keychain.Fake, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".credentials.json")
	kc := keychain.NewFake()
	s := New(kc, path)
	return s, kc, path
}

// TestFileOnlyMode covers the non-macOS path: with no OS keychain (nil kc), the
// live store is file-only, mirroring what Claude Code does on Linux/Windows.
func TestFileOnlyMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".credentials.json")
	s := New(nil, path) // nil keychain = file is the sole store

	in := cred(time.Now().Add(time.Hour).UnixMilli(), "tok-linux")
	if err := s.Write(in); err != nil {
		t.Fatalf("Write (file-only): %v", err)
	}
	got, err := s.Read()
	if err != nil {
		t.Fatalf("Read (file-only): %v", err)
	}
	if got.AccessToken != "tok-linux" {
		t.Errorf("read back %q, want tok-linux", got.AccessToken)
	}

	// sync status: file authoritative, keychain absent but that is not a desync.
	rep, err := s.SyncStatus()
	if err != nil {
		t.Fatalf("SyncStatus: %v", err)
	}
	if rep.KeyPresent {
		t.Error("KeyPresent should be false with no keychain")
	}
	if !rep.FilePresent {
		t.Error("FilePresent should be true after a write")
	}
	if !rep.InSync {
		t.Error("file-only store with a present file should report in sync")
	}
}

func cred(expMs int64, access string) credential.Credential {
	return credential.Credential{
		AccessToken:      access,
		RefreshToken:     "r-" + access,
		ExpiresAt:        expMs,
		Scopes:           []string{"user:profile"},
		SubscriptionType: "max",
	}
}

func TestWriteThenRead(t *testing.T) {
	s, kc, path := newTestStore(t)
	in := cred(time.Now().Add(time.Hour).UnixMilli(), "tok1")
	if err := s.Write(in); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// file exists, 0600, wrapped under claudeAiOauth
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %v, want 0600", info.Mode().Perm())
	}
	raw, _ := os.ReadFile(path)
	var wrap map[string]json.RawMessage
	_ = json.Unmarshal(raw, &wrap)
	if _, ok := wrap["claudeAiOauth"]; !ok {
		t.Errorf("file not wrapped under claudeAiOauth: %s", raw)
	}
	// keychain also got it
	if names, _ := kc.List(LiveService); len(names) == 0 {
		t.Error("keychain entry not written")
	}
	// read back
	got, err := s.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.AccessToken != "tok1" {
		t.Errorf("read access = %q", got.AccessToken)
	}
}

func TestReadPrefersFreshestOnDesync(t *testing.T) {
	s, kc, path := newTestStore(t)
	// file has an OLD token, keychain has a NEWER one (the known desync case)
	oldC := cred(time.Now().Add(1*time.Hour).UnixMilli(), "old")
	newC := cred(time.Now().Add(5*time.Hour).UnixMilli(), "new")
	writeFileOnly(t, path, oldC)
	writeKeychainOnly(t, kc, newC)

	got, err := s.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.AccessToken != "new" {
		t.Errorf("expected freshest (new) token, got %q", got.AccessToken)
	}
}

func TestSyncStatusDetectsMismatch(t *testing.T) {
	s, kc, path := newTestStore(t)
	writeFileOnly(t, path, cred(1000, "a"))
	writeKeychainOnly(t, kc, cred(2000, "b"))
	rep, err := s.SyncStatus()
	if err != nil {
		t.Fatalf("SyncStatus: %v", err)
	}
	if rep.InSync {
		t.Error("expected InSync=false for differing tokens")
	}
	// after a Write both sides agree
	_ = s.Write(cred(3000, "c"))
	rep, _ = s.SyncStatus()
	if !rep.InSync {
		t.Error("expected InSync=true after Write")
	}
}

// helpers that poke one side only, to simulate desync

func writeFileOnly(t *testing.T, path string, c credential.Credential) {
	t.Helper()
	raw, _ := json.Marshal(map[string]credential.Credential{"claudeAiOauth": c})
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeKeychainOnly(t *testing.T, kc *keychain.Fake, c credential.Credential) {
	t.Helper()
	raw, _ := json.Marshal(map[string]credential.Credential{"claudeAiOauth": c})
	if err := kc.Set(LiveService, "tester", raw); err != nil {
		t.Fatal(err)
	}
}
