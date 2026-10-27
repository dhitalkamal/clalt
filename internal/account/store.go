package account

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dhitalkamal/clalt/internal/fslock"
	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// lockTimeout bounds the wait for another clalt process that is saving
// state.json. a save takes milliseconds, so waiting this long means a stuck
// process, and the error says so.
const lockTimeout = 10 * time.Second

// state is the on-disk metadata file (no secrets). active is a pointer that
// older versions set when they switched accounts. nothing sets it now, but it is
// still written back so that the v0.1.0 binary can read the file.
type state struct {
	Active   string          `json:"active"`
	Accounts map[string]Meta `json:"accounts"`

	// when the first sign-in of the folder design worked, see Store.ThinSince.
	// the v0.1.0 binary ignores a key it does not know
	ThinSince time.Time `json:"thinSince,omitempty,omitzero"`

	// the folders each account may serve and whether auto-switch may use it, by
	// account id. it is a map of its own and not a field of Meta so that Meta stays
	// a value that can be compared with ==, which tests of several commands use to
	// check that a refused command changed nothing. v0.1.0 ignores the key.
	Rules map[string]Rules `json:"rules,omitempty"`
}

// Store is the account manager.
type Store struct {
	statePath string
	logPath   string
	now       func() time.Time // injectable clock for tests
}

// New builds a Store.
func New(statePath, logPath string) *Store {
	return &Store{statePath: statePath, logPath: logPath, now: time.Now}
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

// ThinSince is the time of the first sign-in that worked since accounts became
// folders, or zero when none has. the old switch log stopped growing then, so
// usage from that time on has no account to be charged to.
func (s *Store) ThinSince() (time.Time, error) {
	st, err := s.load()
	return st.ThinSince, err
}

// List returns all accounts sorted by id.
func (s *Store) List() ([]Account, error) {
	st, err := s.load()
	if err != nil {
		return nil, err
	}
	out := make([]Account, 0, len(st.Accounts))
	for name, meta := range st.Accounts {
		out = append(out, Account{Name: name, Meta: meta})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// mutate runs one load-modify-save cycle under a file lock, so parallel clalt
// processes (several `clalt run` sessions) cannot overwrite each other's
// changes. fn must not call another method that mutates: the lock is not
// re-entrant. nothing is written when fn returns an error.
func (s *Store) mutate(fn func(st *state) error) error {
	if err := checkPath(s.statePath); err != nil {
		return err
	}
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

// checkPath refuses a path that is not absolute. with no home the paths of the
// state and of the switch log are relative, which means the working directory,
// and a file there is somebody else's and one written there would be lost.
func checkPath(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("clalt's state is at %q, which is not an absolute path: is HOME set?", path)
	}
	return nil
}

// load reads state.json, returning an empty state if the file is absent.
func (s *Store) load() (state, error) {
	st := state{Accounts: map[string]Meta{}}
	if err := checkPath(s.statePath); err != nil {
		return st, err
	}
	raw, err := fsutil.ReadFile(s.statePath)
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
