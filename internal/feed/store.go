package feed

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/fslock"
	"github.com/dhitalkamal/clalt/internal/fsutil"
)

const defaultLockWait = 2 * time.Second

// endedKeep is how long the record of an ended session stays, so that a reader
// that polls sees the end and not only that the file went away.
const endedKeep = 10 * time.Minute

// Store is the feed folder. LockWait is how long a writer waits for the one
// lock of the folder, 2 seconds when zero; a hook sets a short one, since a
// skipped update costs less than a stalled claude.
type Store struct {
	Dir      string
	LockWait time.Duration
}

func (s Store) sessionPath(handle string) string {
	return filepath.Join(s.Dir, "sessions", handle+".json")
}

func (s Store) lock() (func(), error) {
	wait := s.LockWait
	if wait <= 0 {
		wait = defaultLockWait
	}
	l, err := fslock.Acquire(filepath.Join(s.Dir, ".lock"), wait)
	if err != nil {
		return nil, err
	}
	return func() { _ = l.Release() }, nil
}

func writeJSON(path string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return fsutil.WriteFileAtomicNoSync(path, raw, 0o600)
}

func readJSON(path string, v any) bool {
	raw, err := fsutil.ReadFile(path)
	return err == nil && json.Unmarshal(raw, v) == nil
}

// UpdateSession changes the state of a session under the lock. a session with
// no file yet starts as "starting". fn says whether it changed anything; only
// then is the file written, with UpdatedAt set to now.
func (s Store) UpdateSession(handle, account string, now time.Time, fn func(*SessionState) bool) error {
	if !ValidKey(handle) {
		return fmt.Errorf("not a session handle: %q", handle)
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	st, ok := s.Session(handle)
	if !ok {
		st = SessionState{Handle: handle, Account: account, State: StateStarting, Since: now}
	}
	if !fn(&st) {
		return nil
	}
	st.UpdatedAt = now
	return writeJSON(s.sessionPath(handle), st)
}

// Session reads one session. a missing or corrupt file is "not there".
func (s Store) Session(handle string) (SessionState, bool) {
	var st SessionState
	if !ValidKey(handle) || !readJSON(s.sessionPath(handle), &st) {
		return SessionState{}, false
	}
	return st, true
}

// Sessions reads every session that can be read, ordered by handle.
func (s Store) Sessions() ([]SessionState, error) {
	files, err := os.ReadDir(filepath.Join(s.Dir, "sessions"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []SessionState
	for _, f := range files {
		if h, ok := strings.CutSuffix(f.Name(), ".json"); ok {
			if st, ok := s.Session(h); ok {
				out = append(out, st)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Handle < out[j].Handle })
	return out, nil
}

// RemoveSession deletes the file of a session. a missing one is not an error.
func (s Store) RemoveSession(handle string) error {
	if !ValidKey(handle) {
		return nil
	}
	if err := os.Remove(s.sessionPath(handle)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Prune removes the sessions that are not live, except an ended one that is
// younger than ten minutes. live is asked about each handle, so feed does not
// need to know where liveness comes from. it returns how many were removed.
func (s Store) Prune(now time.Time, live func(handle string) bool) (int, error) {
	if _, err := os.Stat(s.Dir); err != nil {
		return 0, nil
	}
	unlock, err := s.lock()
	if err != nil {
		return 0, err
	}
	defer unlock()
	all, err := s.Sessions()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, st := range all {
		if live(st.Handle) || (st.State == StateEnded && now.Sub(st.UpdatedAt) < endedKeep) {
			continue
		}
		if err := s.RemoveSession(st.Handle); err != nil {
			return n, err
		}
		_ = os.Remove(s.activityPath(st.Handle))
		n++
	}
	return n, nil
}

func (s Store) activityPath(handle string) string {
	return filepath.Join(s.Dir, "sessions", handle+".act")
}

// TouchActivity notes that a tool of the session finished at now. it takes no
// lock, since it runs after every tool call and a lost write only makes a
// later notification look older than it is.
func (s Store) TouchActivity(handle string, now time.Time) error {
	if !ValidKey(handle) {
		return fmt.Errorf("not a session handle: %q", handle)
	}
	return writeJSON(s.activityPath(handle), now.UTC())
}

// Activity is when TouchActivity was last called for the session, or zero.
func (s Store) Activity(handle string) time.Time {
	var t time.Time
	if !ValidKey(handle) || !readJSON(s.activityPath(handle), &t) {
		return time.Time{}
	}
	return t
}
