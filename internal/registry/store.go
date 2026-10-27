package registry

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

// Registry is the folder of session records. the zero LockWait means 2 seconds;
// a hook sets a shorter one, since it must not hold claude up.
type Registry struct {
	Dir      string
	LockWait time.Duration
}

// Entry is a record as found on disk. File is the name of the file, which is
// not the handle for a record that an earlier version wrote.
type Entry struct {
	Record
	File  string
	Alive bool
}

func (r Registry) path(handle string) string { return filepath.Join(r.Dir, handle+".json") }

// one lock for the whole folder: the sections are microseconds, and there are
// no per-record lock files to clean up.
func (r Registry) lock() (func(), error) {
	wait := r.LockWait
	if wait <= 0 {
		wait = defaultLockWait
	}
	l, err := fslock.Acquire(filepath.Join(r.Dir, ".lock"), wait)
	if err != nil {
		return nil, err
	}
	return func() { _ = l.Release() }, nil
}

func (r Registry) write(rec Record) error {
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomicNoSync(r.path(rec.Handle), raw, 0o600)
}

// Create writes the record of a session.
func (r Registry) Create(rec Record) error {
	if !ValidHandle(rec.Handle) {
		return fmt.Errorf("not a session handle: %q", rec.Handle)
	}
	if err := os.MkdirAll(r.Dir, 0o700); err != nil {
		return err
	}
	unlock, err := r.lock()
	if err != nil {
		return err
	}
	defer unlock()
	return r.write(rec)
}

func read(path string) (Record, error) {
	raw, err := fsutil.ReadFile(path)
	if err != nil {
		return Record{}, err
	}
	var rec Record
	return rec, json.Unmarshal(raw, &rec)
}

// Get finds a record by handle. a bad handle or a missing file is "not there":
// the caller is a hook that has nothing to say about it.
func (r Registry) Get(handle string) (Record, bool, error) {
	if !ValidHandle(handle) {
		return Record{}, false, nil
	}
	rec, err := read(r.path(handle))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Record{}, false, nil
	case err != nil:
		return Record{}, false, err
	}
	return rec, true, nil
}

// Update changes a record under the lock. a record that is not there stays not
// there, so a hook that comes late cannot bring back a session that ended.
func (r Registry) Update(handle string, fn func(*Record)) (Record, bool, error) {
	if !ValidHandle(handle) {
		return Record{}, false, nil
	}
	if _, err := os.Stat(r.Dir); err != nil {
		return Record{}, false, nil
	}
	unlock, err := r.lock()
	if err != nil {
		return Record{}, false, err
	}
	defer unlock()
	rec, ok, err := r.Get(handle)
	if err != nil || !ok {
		return Record{}, false, err
	}
	fn(&rec)
	rec.Handle = handle
	return rec, true, r.write(rec)
}

// Remove deletes the record of a session. a missing one is not an error.
func (r Registry) Remove(handle string) error {
	if !ValidHandle(handle) {
		return nil
	}
	return r.removeFile(r.path(handle))
}

func (r Registry) removeFile(path string) error {
	if _, err := os.Stat(r.Dir); err != nil {
		return nil
	}
	unlock, err := r.lock()
	if err != nil {
		return err
	}
	defer unlock()
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Alive says whether the process the record is tied to exists: the wrapper, or
// for a record of an earlier version, which has none, claude.
func (rec Record) Alive() bool {
	if rec.WrapperPID > 0 {
		return pidAlive(rec.WrapperPID)
	}
	return pidAlive(rec.PID)
}

// Scan reads every record and changes nothing. bad counts the files that could
// not be read as a record.
func (r Registry) Scan() (entries []Entry, bad int, err error) {
	files, err := os.ReadDir(r.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") || strings.HasPrefix(f.Name(), ".") {
			continue
		}
		rec, err := read(filepath.Join(r.Dir, f.Name()))
		if err != nil {
			bad++
			continue
		}
		entries = append(entries, Entry{Record: rec, File: f.Name(), Alive: rec.Alive()})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].StartedAt.Before(entries[j].StartedAt) })
	return entries, bad, nil
}

// List returns the live records, oldest first, and removes the files of the
// ones whose process is gone.
func (r Registry) List() ([]Record, error) {
	entries, _, err := r.Scan()
	if err != nil {
		return nil, err
	}
	var out []Record
	for _, e := range entries {
		if e.Alive {
			out = append(out, e.Record)
			continue
		}
		_ = r.removeFile(filepath.Join(r.Dir, e.File))
	}
	return out, nil
}

// RemovePID removes the records that belong to a process, claude's or the
// wrapper's.
func (r Registry) RemovePID(pid int) error {
	entries, _, err := r.Scan()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.PID == pid || e.WrapperPID == pid {
			if err := r.removeFile(filepath.Join(r.Dir, e.File)); err != nil {
				return err
			}
		}
	}
	return nil
}
