package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dhitalkamal/clalt/internal/credential"
)

func TestStageAndRead(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "work")

	if HasCredential(dir) {
		t.Error("fresh dir should have no credential")
	}
	if _, ok, err := Read(dir); ok || err != nil {
		t.Errorf("Read fresh = ok %v err %v", ok, err)
	}

	c := credential.Credential{AccessToken: "tok", RefreshToken: "r", ExpiresAt: 123, SubscriptionType: "max"}
	if err := Stage(dir, c); err != nil {
		t.Fatalf("Stage: %v", err)
	}

	// file exists, 0600, wrapped under claudeAiOauth
	info, err := os.Stat(CredPath(dir))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
	if !HasCredential(dir) {
		t.Error("HasCredential should be true after Stage")
	}

	got, ok, err := Read(dir)
	if err != nil || !ok {
		t.Fatalf("Read: ok %v err %v", ok, err)
	}
	if got.AccessToken != "tok" || got.RefreshToken != "r" {
		t.Errorf("round trip wrong: %+v", got)
	}
}
