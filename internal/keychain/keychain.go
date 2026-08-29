// Package keychain abstracts a secret store keyed by (service, account) so the
// rest of clalt can be tested without touching the real macOS Keychain.
package keychain

import (
	"errors"
	"os"
)

// ErrNotFound is returned when a requested item does not exist.
var ErrNotFound = errors.New("keychain: item not found")

// EnvSandboxDir, when set, makes NewSystem use a file-backed sandbox store rooted
// at that directory instead of the macOS Keychain. This exists for testing and
// end-to-end runs; it is NOT the production backend and does not encrypt at rest.
const EnvSandboxDir = "CLALT_KEYCHAIN_DIR"

// NewSystem returns the native OS credential store: the macOS Keychain on
// darwin, or nil off macOS (Linux/Windows have no keychain clalt needs; the
// live credential is file-based there). When CLALT_KEYCHAIN_DIR is set it
// returns a file-backed sandbox store instead, so mutating flows can be
// exercised on any platform without touching a real Keychain. A nil return is
// valid and means "no native keychain"; callers must be nil-safe.
func NewSystem() Store {
	if dir := os.Getenv(EnvSandboxDir); dir != "" {
		return NewFileStore(dir)
	}
	return newNative()
}

// Store is the minimal secret-store surface clalt needs.
type Store interface {
	// Get returns the data for (service, account) or ErrNotFound.
	Get(service, account string) ([]byte, error)
	// Set creates or replaces the data for (service, account).
	Set(service, account string, data []byte) error
	// Delete removes (service, account); missing items are not an error.
	Delete(service, account string) error
	// List returns all account names stored under service.
	List(service string) ([]string, error)
}
