// Package backup seals and restores all clalt accounts as a single encrypted
// file so they can move between machines. Encryption is passphrase -> argon2id
// -> XSalsa20-Poly1305 (nacl/secretbox).
package backup

import (
	"crypto/rand"
	"encoding/json"
	"errors"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/nacl/secretbox"
)

// file layout: magic(4) | salt(16) | nonce(24) | ciphertext
const magic = "CLB1"

const (
	saltLen  = 16
	nonceLen = 24
	keyLen   = 32
)

// argon2id parameters (interactive-ish; a backup is not hot-path).
const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 4
)

// Item is one account's secret + metadata as raw JSON.
type Item struct {
	Credential json.RawMessage `json:"credential"`
	Meta       json.RawMessage `json:"meta"`
}

// Bundle is the full set of accounts to export.
type Bundle struct {
	Accounts map[string]Item `json:"accounts"`
}

// Marshal serializes a bundle to JSON.
func (b Bundle) Marshal() ([]byte, error) { return json.Marshal(b) }

// ParseBundle deserializes bundle JSON.
func ParseBundle(raw []byte) (Bundle, error) {
	var b Bundle
	err := json.Unmarshal(raw, &b)
	return b, err
}

func deriveKey(passphrase string, salt []byte) [keyLen]byte {
	k := argon2.IDKey([]byte(passphrase), salt, argonTime, argonMemory, argonThreads, keyLen)
	var key [keyLen]byte
	copy(key[:], k)
	return key
}

// Seal encrypts plaintext under passphrase.
func Seal(plaintext []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	var nonce [nonceLen]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	key := deriveKey(passphrase, salt)

	out := make([]byte, 0, len(magic)+saltLen+nonceLen+len(plaintext)+secretbox.Overhead)
	out = append(out, magic...)
	out = append(out, salt...)
	out = append(out, nonce[:]...)
	out = secretbox.Seal(out, plaintext, &nonce, &key)
	return out, nil
}

// Open decrypts a sealed blob under passphrase.
func Open(sealed []byte, passphrase string) ([]byte, error) {
	header := len(magic) + saltLen + nonceLen
	if len(sealed) < header+secretbox.Overhead {
		return nil, errors.New("backup: file too short or corrupt")
	}
	if string(sealed[:len(magic)]) != magic {
		return nil, errors.New("backup: bad magic (not a clalt export)")
	}
	off := len(magic)
	salt := sealed[off : off+saltLen]
	off += saltLen
	var nonce [nonceLen]byte
	copy(nonce[:], sealed[off:off+nonceLen])
	off += nonceLen
	ciphertext := sealed[off:]

	key := deriveKey(passphrase, salt)
	plain, ok := secretbox.Open(nil, ciphertext, &nonce, &key)
	if !ok {
		return nil, errors.New("backup: wrong passphrase or corrupt file")
	}
	return plain, nil
}
