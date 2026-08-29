package keychain

import (
	"path/filepath"
	"testing"
)

func TestMigrateVaultCopiesLegacySecrets(t *testing.T) {
	src := NewFake()
	_ = src.Set("clalt", "work", []byte("tok-work"))
	_ = src.Set("clalt", "home", []byte("tok-home"))

	dir := t.TempDir()
	dst, err := NewEncFileStore(filepath.Join(dir, "vault"), filepath.Join(dir, "vault.key"))
	if err != nil {
		t.Fatal(err)
	}

	n, err := MigrateVault(src, dst, "clalt")
	if err != nil {
		t.Fatalf("MigrateVault: %v", err)
	}
	if n != 2 {
		t.Errorf("migrated %d, want 2", n)
	}
	got, err := dst.Get("clalt", "work")
	if err != nil || string(got) != "tok-work" {
		t.Errorf("work not migrated: %q %v", got, err)
	}
}

func TestMigrateVaultSkipsWhenDestNotEmpty(t *testing.T) {
	src := NewFake()
	_ = src.Set("clalt", "work", []byte("tok-work"))

	dir := t.TempDir()
	dst, err := NewEncFileStore(filepath.Join(dir, "vault"), filepath.Join(dir, "vault.key"))
	if err != nil {
		t.Fatal(err)
	}
	// dst already has an item -> migration must be a no-op
	_ = dst.Set("clalt", "existing", []byte("keep"))

	n, err := MigrateVault(src, dst, "clalt")
	if err != nil {
		t.Fatalf("MigrateVault: %v", err)
	}
	if n != 0 {
		t.Errorf("migrated %d, want 0 (dest not empty)", n)
	}
	if _, err := dst.Get("clalt", "work"); err == nil {
		t.Error("should not have copied when dest non-empty")
	}
}
