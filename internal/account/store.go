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
	"github.com/dhitalkamal/clalt/internal/keychain"
	"github.com/dhitalkamal/clalt/internal/livestore"
)

// state is the on-disk metadata file (no secrets).
type state struct {
	Active    string          `json:"active"`
	Accounts  map[string]Meta `json:"accounts"`
	Fallback  []string        `json:"fallback,omitempty"`  // ordered rotation chain
	Threshold float64         `json:"threshold,omitempty"` // util fraction that triggers rotation
}

// DefaultThreshold is the utilization fraction above which `next --live` rotates
// off an account.
const DefaultThreshold = 0.90

// Store is the account manager.
type Store struct {
	kc        keychain.Store
	live      *livestore.Store
	statePath string
	logPath   string
	now       func() time.Time // injectable clock for tests
}

// New builds a Store.
func New(kc keychain.Store, live *livestore.Store, statePath, logPath string) *Store {
	return &Store{kc: kc, live: live, statePath: statePath, logPath: logPath, now: time.Now}
}

// Save stores (or replaces) a slot's credential and upserts its metadata.
// AddedAt is set only on first save; a later save preserves it.
func (s *Store) Save(name string, c credential.Credential, meta Meta) error {
	if name == "" {
		return errors.New("account name is empty")
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := s.kc.Set(KeychainService, name, raw); err != nil {
		return fmt.Errorf("store secret: %w", err)
	}
	st, err := s.load()
	if err != nil {
		return err
	}
	existing, ok := st.Accounts[name]
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
	return s.save(st)
}

// SetBudget sets a slot's daily and/or weekly spend caps (USD). A negative value
// leaves that cap unchanged; zero clears it.
func (s *Store) SetBudget(name string, daily, weekly float64) error {
	st, err := s.load()
	if err != nil {
		return err
	}
	meta, ok := st.Accounts[name]
	if !ok {
		return fmt.Errorf("account %q not found", name)
	}
	if daily >= 0 {
		meta.DailyBudget = daily
	}
	if weekly >= 0 {
		meta.WeeklyBudget = weekly
	}
	st.Accounts[name] = meta
	return s.save(st)
}

// Get returns a slot's credential and metadata.
func (s *Store) Get(name string) (credential.Credential, Meta, error) {
	raw, err := s.kc.Get(KeychainService, name)
	if err != nil {
		return credential.Credential{}, Meta{}, fmt.Errorf("account %q not found", name)
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
	st, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := st.Accounts[name]; !ok {
		return fmt.Errorf("account %q not found", name)
	}
	st.Active = name
	return s.save(st)
}

// Remove deletes a slot. It refuses to remove the active account so a switch is
// required first (prevents leaving Claude Code pointed at a deleted slot).
func (s *Store) Remove(name string) error {
	st, err := s.load()
	if err != nil {
		return err
	}
	if name == st.Active {
		return fmt.Errorf("%q is active; switch to another account before removing it", name)
	}
	if _, ok := st.Accounts[name]; !ok {
		return fmt.Errorf("account %q not found", name)
	}
	if err := s.kc.Delete(KeychainService, name); err != nil {
		return err
	}
	delete(st.Accounts, name)
	return s.save(st)
}

// Delete removes a slot unconditionally (unlike Remove, it does not guard the
// active account); it clears the active pointer if it pointed here. Used by
// logout, where forgetting the active account is the explicit intent.
func (s *Store) Delete(name string) error {
	st, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := st.Accounts[name]; !ok {
		return fmt.Errorf("account %q not found", name)
	}
	if err := s.kc.Delete(KeychainService, name); err != nil {
		return err
	}
	delete(st.Accounts, name)
	if st.Active == name {
		st.Active = ""
	}
	return s.save(st)
}

// Rename moves a slot to a new name, fixing the active pointer if needed.
func (s *Store) Rename(oldName, newName string) error {
	if newName == "" {
		return errors.New("new name is empty")
	}
	st, err := s.load()
	if err != nil {
		return err
	}
	meta, ok := st.Accounts[oldName]
	if !ok {
		return fmt.Errorf("account %q not found", oldName)
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
	return s.save(st)
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
	if len(raw) == 0 {
		return st, nil
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
	return os.WriteFile(s.statePath, raw, 0o600)
}
