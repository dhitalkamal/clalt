// Package keychain abstracts a secret store keyed by (service, account). clalt
// no longer uses any OS keychain: the production store is EncFileStore (a local
// encrypted vault), Store exists so tests can swap in an in-memory fake.
package keychain

import "errors"

// ErrNotFound is returned when a requested item does not exist.
var ErrNotFound = errors.New("keychain: item not found")

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
