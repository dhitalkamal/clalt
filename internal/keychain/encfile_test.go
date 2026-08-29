package keychain

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func newEnc(t *testing.T) (*EncFileStore, string) {
	t.Helper()
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "vault.key")
	s, err := NewEncFileStore(filepath.Join(dir, "vault"), keyPath)
	if err != nil {
		t.Fatalf("NewEncFileStore: %v", err)
	}
	return s, keyPath
}

func TestEncFileRoundTrip(t *testing.T) {
	s, _ := newEnc(t)
	secret := []byte(`{"accessToken":"super-secret"}`)
	if err := s.Set("clalt", "work", secret); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get("clalt", "work")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, secret) {
		t.Errorf("Get = %q, want %q", got, secret)
	}
}

func TestEncFileMissingIsNotFound(t *testing.T) {
	s, _ := newEnc(t)
	if _, err := s.Get("clalt", "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing item err = %v, want ErrNotFound", err)
	}
}

func TestEncFileDataOnDiskIsEncrypted(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "vault.key")
	vaultDir := filepath.Join(dir, "vault")
	s, err := NewEncFileStore(vaultDir, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	secret := []byte("plaintext-token-value")
	if err := s.Set("clalt", "work", secret); err != nil {
		t.Fatal(err)
	}
	// walk the vault dir and assert no file contains the plaintext
	err = filepath.Walk(vaultDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if bytes.Contains(raw, secret) {
			t.Errorf("plaintext secret found on disk at %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEncFileKeyPersistsAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "vault.key")
	vaultDir := filepath.Join(dir, "vault")

	s1, err := NewEncFileStore(vaultDir, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s1.Set("clalt", "work", []byte("tok")); err != nil {
		t.Fatal(err)
	}
	// a fresh instance reusing the same key file must read the same data
	s2, err := NewEncFileStore(vaultDir, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s2.Get("clalt", "work")
	if err != nil || string(got) != "tok" {
		t.Errorf("second instance Get = %q, %v", got, err)
	}
}

func TestEncFileWrongKeyFailsToDecrypt(t *testing.T) {
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, "vault")

	s1, err := NewEncFileStore(vaultDir, filepath.Join(dir, "keyA"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s1.Set("clalt", "work", []byte("tok")); err != nil {
		t.Fatal(err)
	}
	// point a new store at the same data but a different key
	s2, err := NewEncFileStore(vaultDir, filepath.Join(dir, "keyB"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s2.Get("clalt", "work"); err == nil {
		t.Error("decryption with wrong key should fail")
	}
}

func TestEncFileListAndDelete(t *testing.T) {
	s, _ := newEnc(t)
	_ = s.Set("clalt", "work", []byte("a"))
	_ = s.Set("clalt", "home", []byte("b"))
	names, err := s.List("clalt")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Errorf("List = %v, want 2 names", names)
	}
	if err := s.Delete("clalt", "work"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("clalt", "work"); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted item still present: %v", err)
	}
}
