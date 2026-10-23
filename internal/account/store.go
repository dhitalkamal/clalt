package account

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dhitalkamal/clalt/internal/credential"
	"github.com/dhitalkamal/clalt/internal/fslock"
	"github.com/dhitalkamal/clalt/internal/fsutil"
	"github.com/dhitalkamal/clalt/internal/keychain"
)

// lockTimeout bounds the wait for another clalt process that is saving
// state.json. a save takes milliseconds, so waiting this long means a stuck
// process, and the error says so.
const lockTimeout = 10 * time.Second

// state is the on-disk metadata file (no secrets).
type state struct {
	Active   string          `json:"active"`
	Accounts map[string]Meta `json:"accounts"`
}

// Store is the account manager.
type Store struct {
	kc        keychain.Store
	statePath string
	logPath   string
	now       func() time.Time // injectable clock for tests
}

// New builds a Store.
func New(kc keychain.Store, statePath, logPath string) *Store {
	return &Store{kc: kc, statePath: statePath, logPath: logPath, now: time.Now}
}

// Save stores (or replaces) a slot's credential and upserts its metadata.
// AddedAt is set only on first save; a later save preserves it. A brand-new
// name is validated (this is the "creating an account" boundary: add, login,
// import); re-saving an already-known name (refresh, sync-back) is not
// re-validated, so an account added before ValidateName existed keeps working
// until the user explicitly renames it.
func (s *Store) Save(name string, c credential.Credential, meta Meta) error {
	if name == "" {
		return errors.New("account name is empty")
	}
	return s.mutate(func(st *state) error {
		existing, ok := st.Accounts[name]
		if !ok {
			if verr := ValidateName(name); verr != nil {
				return verr
			}
		}
		raw, err := json.Marshal(c)
		if err != nil {
			return err
		}
		if err := s.kc.Set(KeychainService, name, raw); err != nil {
			return fmt.Errorf("store secret: %w", err)
		}
		if ok && !existing.AddedAt.IsZero() {
			meta.AddedAt = existing.AddedAt
		} else {
			meta.AddedAt = s.now()
		}
		// keep the previous lastUsedAt unless the caller set one
		if meta.LastUsedAt.IsZero() {
			meta.LastUsedAt = existing.LastUsedAt
		}
		st.Accounts[name] = meta
		return nil
	})
}

// SetBudget sets a slot's daily and/or weekly spend caps (USD). A negative value
// leaves that cap unchanged; zero clears it.
func (s *Store) SetBudget(name string, daily, weekly float64) error {
	return s.mutate(func(st *state) error {
		meta, ok := st.Accounts[name]
		if !ok {
			return errNoAccount(name)
		}
		if daily >= 0 {
			meta.DailyBudget = daily
		}
		if weekly >= 0 {
			meta.WeeklyBudget = weekly
		}
		st.Accounts[name] = meta
		return nil
	})
}

// Get returns a slot's credential and metadata.
func (s *Store) Get(name string) (credential.Credential, Meta, error) {
	raw, err := s.kc.Get(KeychainService, name)
	if err != nil {
		return credential.Credential{}, Meta{}, errNoAccount(name)
	}
	var c credential.Credential
	if err := json.Unmarshal(raw, &c); err != nil {
		return credential.Credential{}, Meta{}, fmt.Errorf("decode secret for %q: %w", name, err)
	}
	st, err := s.load()
	if err != nil {
		return credential.Credential{}, Meta{}, err
	}
	return c, st.Accounts[name], nil
}

// List returns all accounts sorted by name with the active one flagged.
func (s *Store) List() ([]Account, error) {
	st, err := s.load()
	if err != nil {
		return nil, err
	}
	out := make([]Account, 0, len(st.Accounts))
	for name, meta := range st.Accounts {
		out = append(out, Account{Name: name, Meta: meta, Active: name == st.Active})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Active returns the active account name (empty if none).
func (s *Store) Active() (string, error) {
	st, err := s.load()
	if err != nil {
		return "", err
	}
	return st.Active, nil
}

// SetActive points the active marker at name (must exist).
func (s *Store) SetActive(name string) error {
	return s.mutate(func(st *state) error {
		if _, ok := st.Accounts[name]; !ok {
			return errNoAccount(name)
		}
		st.Active = name
		return nil
	})
}

// Remove deletes a slot. It refuses to remove the active account so a switch is
// required first (prevents leaving Claude Code pointed at a deleted slot).
func (s *Store) Remove(name string) error {
	return s.mutate(func(st *state) error {
		if name == st.Active {
			return fmt.Errorf("%q is active; switch to another account before removing it", name)
		}
		if _, ok := st.Accounts[name]; !ok {
			return errNoAccount(name)
		}
		if err := s.kc.Delete(KeychainService, name); err != nil {
			return err
		}
		delete(st.Accounts, name)
		return nil
	})
}

// Delete removes a slot unconditionally (unlike Remove, it does not guard the
// active account); it clears the active pointer if it pointed here. Used by
// logout, where forgetting the active account is the explicit intent.
func (s *Store) Delete(name string) error {
	return s.mutate(func(st *state) error {
		if _, ok := st.Accounts[name]; !ok {
			return errNoAccount(name)
		}
		if err := s.kc.Delete(KeychainService, name); err != nil {
			return err
		}
		delete(st.Accounts, name)
		if st.Active == name {
			st.Active = ""
		}
		return nil
	})
}

// Rename moves a slot to a new name, fixing the active pointer if needed. Only
// newName is validated: it becomes a fresh storage key / profile dir name, so
// it must be safe, but oldName is not re-validated, which is what lets an
// account with a pre-validation legacy name be renamed out of that state.
func (s *Store) Rename(oldName, newName string) error {
	if newName == "" {
		return errors.New("new name is empty")
	}
	if err := ValidateName(newName); err != nil {
		return err
	}
	return s.mutate(func(st *state) error {
		meta, ok := st.Accounts[oldName]
		if !ok {
			return errNoAccount(oldName)
		}
		if _, exists := st.Accounts[newName]; exists {
			return fmt.Errorf("account %q already exists", newName)
		}
		raw, err := s.kc.Get(KeychainService, oldName)
		if err != nil {
			return fmt.Errorf("read secret for %q: %w", oldName, err)
		}
		if err := s.kc.Set(KeychainService, newName, raw); err != nil {
			return err
		}
		if err := s.kc.Delete(KeychainService, oldName); err != nil {
			return err
		}
		delete(st.Accounts, oldName)
		st.Accounts[newName] = meta
		if st.Active == oldName {
			st.Active = newName
		}
		return nil
	})
}

// mutate runs one load-modify-save cycle under a file lock, so parallel clalt
// processes (several `clalt run` sessions) cannot overwrite each other's
// changes. fn must not call another method that mutates: the lock is not
// re-entrant. nothing is written when fn returns an error.
func (s *Store) mutate(fn func(st *state) error) error {
	return fslock.WithLock(s.statePath+".lock", lockTimeout, func() error {
		st, err := s.load()
		if err != nil {
			return err
		}
		if err := fn(&st); err != nil {
			return err
		}
		return s.save(st)
	})
}

// load reads state.json, returning an empty state if the file is absent.
func (s *Store) load() (state, error) {
	st := state{Accounts: map[string]Meta{}}
	raw, err := os.ReadFile(s.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, err
	}
	// a missing file is a store with no accounts, but an empty one is a file
	// something cut short, and writing on top of it would hide that
	if len(raw) == 0 {
		return st, fmt.Errorf("%s is empty", s.statePath)
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return st, fmt.Errorf("parse state.json: %w", err)
	}
	if st.Accounts == nil {
		st.Accounts = map[string]Meta{}
	}
	return st, nil
}

func (s *Store) save(st state) error {
	if err := os.MkdirAll(filepath.Dir(s.statePath), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.statePath, raw, 0o600)
}
