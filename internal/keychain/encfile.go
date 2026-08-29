package keychain

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// EncFileStore is a file-backed Store that encrypts each item at rest with
// AES-256-GCM. The key is a 32-byte value kept in a local key file (chmod 0600),
// created on first use. This is clalt's production per-account vault: unlike the
// macOS Keychain it never prompts for the login password, which matters for the
// background keepalive agent and for repeated reads across many accounts.
//
// Trust boundary: the key file sits next to the vault under the user's config
// dir, so anyone who can read the user's account can decrypt it. That is the
// same boundary Claude Code itself uses (it stores the live token in plaintext
// under the config dir), so this does not weaken the effective posture.
type EncFileStore struct {
	fs  *FileStore
	gcm cipher.AEAD
}

// NewEncFileStore roots an encrypted store at dir, using (or creating) the key
// at keyPath.
func NewEncFileStore(dir, keyPath string) (*EncFileStore, error) {
	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("vault cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("vault gcm: %w", err)
	}
	return &EncFileStore{fs: NewFileStore(dir), gcm: gcm}, nil
}

func (e *EncFileStore) Get(service, account string) ([]byte, error) {
	raw, err := e.fs.Get(service, account)
	if err != nil {
		return nil, err // includes ErrNotFound
	}
	ns := e.gcm.NonceSize()
	if len(raw) < ns {
		return nil, errors.New("vault item too short to be valid")
	}
	nonce, ct := raw[:ns], raw[ns:]
	plain, err := e.gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("vault decrypt: %w", err)
	}
	return plain, nil
}

func (e *EncFileStore) Set(service, account string, data []byte) error {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	sealed := e.gcm.Seal(nonce, nonce, data, nil)
	return e.fs.Set(service, account, sealed)
}

func (e *EncFileStore) Delete(service, account string) error {
	return e.fs.Delete(service, account)
}

func (e *EncFileStore) List(service string) ([]string, error) {
	return e.fs.List(service)
}

// loadOrCreateKey reads the 32-byte vault key, generating and persisting one on
// first use.
func loadOrCreateKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		if len(data) != 32 {
			return nil, fmt.Errorf("vault key at %s is malformed (%d bytes, want 32)", path, len(data))
		}
		return data, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}
