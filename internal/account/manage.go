package account

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

// the methods in this file only edit state.json: a thin profile keeps its
// login in its own folder, so there is no secret to store and no keychain call.

// maxNameLen is the longest name ValidateName accepts, which a numbered id has
// to stay inside.
const maxNameLen = 64

const maxIDTries = 1000

// Identity is who claude says is signed in to an account's folder.
type Identity struct {
	Email string
	Org   string
	Plan  string
	// zero means now
	SignedInAt time.Time
}

// Create adds an account for label. the id, which names its folder, is the label
// or, when that is taken, label-2, label-3 and so on. an id is taken by any
// account that has it, including one since relabeled and one that is hidden, and
// by a folder that already exists, which the store cannot see for itself, so
// folderExists reports it. it is required, since an id handed out without
// looking could land on someone's folder. a result of "does not exist" in its
// error counts as no folder, so it can be a plain os.Lstat; any other error
// stops the creation. ids compare ignoring case because macOS folders do.
// folderExists runs under the state lock and must not call back into the store.
func (s *Store) Create(label string, folderExists func(id string) (bool, error)) (Account, error) {
	if folderExists == nil {
		return Account{}, errors.New("a check for existing folders is required to create an account")
	}
	if err := ValidateName(label); err != nil {
		return Account{}, err
	}
	var acc Account
	err := s.mutate(func(st *state) error {
		if err := labelHolder(st, label, ""); err != nil {
			return err
		}
		id, err := freeID(st, label, folderExists)
		if err != nil {
			return err
		}
		meta := Meta{Label: label, AddedAt: s.now()}
		st.Accounts[id] = meta
		acc = Account{Name: id, Meta: meta}
		return nil
	})
	if err != nil {
		return Account{}, err
	}
	return acc, nil
}

// ResolveLabel finds the account a label refers to, ignoring case, hidden accounts included
// (check Meta.Hidden). an exact spelling wins over a case-insensitive match;
// two accounts that tie are an error rather than a guess.
func (s *Store) ResolveLabel(label string) (acc Account, found bool, err error) {
	st, err := s.load()
	if err != nil {
		return Account{}, false, err
	}
	ids := make([]string, 0, len(st.Accounts))
	for id := range st.Accounts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var exact, folded []string
	for _, id := range ids {
		switch l := labelOf(id, st.Accounts[id]); {
		case l == label:
			exact = append(exact, id)
		case strings.EqualFold(l, label):
			folded = append(folded, id)
		}
	}
	matches := exact
	if len(matches) == 0 {
		matches = folded
	}
	switch len(matches) {
	case 0:
		return Account{}, false, nil
	case 1:
		id := matches[0]
		return Account{Name: id, Meta: st.Accounts[id]}, true, nil
	}
	return Account{}, false, fmt.Errorf("label %q matches several accounts (ids %s)", label, strings.Join(matches, ", "))
}

// SetIdentity records who signed in. it replaces the whole identity, so an org from an
// earlier login does not outlive a login without one. the first one that works also
// starts ThinSince.
func (s *Store) SetIdentity(id string, in Identity) error {
	if in.Email == "" {
		return errors.New("identity has no email")
	}
	return s.mutate(func(st *state) error {
		m, ok := st.Accounts[id]
		if !ok {
			return errNoAccount(id)
		}
		m.Email, m.Org, m.Plan, m.SignedInAt = in.Email, in.Org, in.Plan, in.SignedInAt
		if m.SignedInAt.IsZero() {
			m.SignedInAt = s.now()
		}
		st.Accounts[id] = m
		if st.ThinSince.IsZero() {
			st.ThinSince = m.SignedInAt
		}
		return nil
	})
}

// ClearSignIn records a logout. the email stays so the next login is still checked against
// it.
func (s *Store) ClearSignIn(id string) error {
	return s.update(id, func(m *Meta) {
		m.Org, m.Plan, m.SignedInAt = "", "", time.Time{}
	})
}

// Hide is what remove does: the account stays, with its folder, label and history, but
// is forgotten as a signed-in identity.
func (s *Store) Hide(id string) error {
	return s.update(id, func(m *Meta) {
		m.Hidden = true
		m.Email, m.Org, m.Plan, m.SignedInAt = "", "", "", time.Time{}
	})
}

// Unhide undoes Hide. the identity is not restored: the next login records a
// fresh one.
func (s *Store) Unhide(id string) error {
	return s.update(id, func(m *Meta) { m.Hidden = false })
}

// Relabel changes the display name and nothing else: the id stays, because claude keys
// the login to the folder it names.
func (s *Store) Relabel(id, label string) error {
	if err := ValidateName(label); err != nil {
		return err
	}
	return s.mutate(func(st *state) error {
		m, ok := st.Accounts[id]
		if !ok {
			return errNoAccount(id)
		}
		if err := labelHolder(st, label, id); err != nil {
			return err
		}
		m.Label = label
		st.Accounts[id] = m
		return nil
	})
}

// Touch records that the account was just used.
func (s *Store) Touch(id string) error {
	return s.update(id, func(m *Meta) { m.LastUsedAt = s.now() })
}

func (s *Store) update(id string, fn func(m *Meta)) error {
	return s.mutate(func(st *state) error {
		m, ok := st.Accounts[id]
		if !ok {
			return errNoAccount(id)
		}
		fn(&m)
		st.Accounts[id] = m
		return nil
	})
}

// labelHolder returns the error for a label that an account other than skip
// already has, or nil when the label is free.
func labelHolder(st *state, label, skip string) error {
	for id, m := range st.Accounts {
		if id != skip && strings.EqualFold(labelOf(id, m), label) {
			return &LabelTakenError{Label: label, ID: id, Hidden: m.Hidden}
		}
	}
	return nil
}

func idTaken(st *state, id string) bool {
	for other := range st.Accounts {
		if strings.EqualFold(other, id) {
			return true
		}
	}
	return false
}

func freeID(st *state, label string, folderExists func(id string) (bool, error)) (string, error) {
	for n := 1; n <= maxIDTries; n++ {
		id := label
		if n > 1 {
			suffix := "-" + strconv.Itoa(n)
			id = label[:min(len(label), maxNameLen-len(suffix))] + suffix
		}
		if idTaken(st, id) {
			continue
		}
		exists, err := folderExists(id)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("check for a folder named %q: %w", id, err)
		}
		if err == nil && exists {
			continue
		}
		if err := ValidateName(id); err != nil {
			return "", err
		}
		return id, nil
	}
	return "", fmt.Errorf("no free id for label %q", label)
}

// CaseCollisions lists the groups of account ids that differ only in case. on
// macOS and Windows such ids name one folder, so two accounts would share a
// login. v0.1.0 never checked for this, so an old state file can hold them, and
// doctor and login report them instead of picking one. each group is sorted,
// and the groups come in alphabetical order of their lowercase id.
func (s *Store) CaseCollisions() ([][]string, error) {
	st, err := s.load()
	if err != nil {
		return nil, err
	}
	byFold := map[string][]string{}
	for id := range st.Accounts {
		key := strings.ToLower(id)
		byFold[key] = append(byFold[key], id)
	}
	var keys []string
	for key, ids := range byFold {
		if len(ids) > 1 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var out [][]string
	for _, key := range keys {
		sort.Strings(byFold[key])
		out = append(out, byFold[key])
	}
	return out, nil
}
