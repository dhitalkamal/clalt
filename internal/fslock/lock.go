// Package fslock provides a cross-process exclusive file lock, used to
// serialize reads and writes of clalt's on-disk state across separate clalt
// invocations (interactive `clalt run` sessions and one-shot commands all run
// in separate OS processes and can race on the same files).
package fslock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// pollInterval is how often a blocked Acquire retries the underlying
// non-blocking lock call while waiting for its turn.
const pollInterval = 25 * time.Millisecond

// Lock is a held exclusive lock on a file. Call Release to give it up.
type Lock struct {
	f *os.File
}

// Acquire opens (creating if needed) the lock file at path and blocks until
// it can take an exclusive lock on it, or returns an error once timeout
// elapses. Every call opens its own file handle, matching how flock/LockFileEx
// actually behave across processes (and, deliberately, across handles within
// one process - see the package-level warning on nested locking).
//
// Nested-locking warning: flock (and LockFileEx) key off the open file
// description, not the process. Calling Acquire twice on the SAME path from
// the SAME goroutine (without releasing the first) will block forever, since
// the second handle can never see the first's lock go away. Callers must
// never call a function that acquires lock X while already holding lock X.
func Acquire(path string, timeout time.Duration) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	deadline := time.Now().Add(timeout)
	for {
		err := tryLock(f)
		if err == nil {
			return &Lock{f: f}, nil
		}
		if !errors.Is(err, errWouldBlock) {
			_ = f.Close()
			return nil, fmt.Errorf("lock %s: %w", path, err)
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, fmt.Errorf("timed out after %s waiting for lock %s (another clalt process is holding it)", timeout, path)
		}
		time.Sleep(pollInterval)
	}
}

// Release gives up the lock and closes its handle.
func (l *Lock) Release() error {
	uerr := unlockFile(l.f)
	cerr := l.f.Close()
	if uerr != nil {
		return uerr
	}
	return cerr
}

// WithLock acquires the lock at path, runs fn while holding it, and always
// releases before returning - even if fn returns an error.
func WithLock(path string, timeout time.Duration, fn func() error) error {
	l, err := Acquire(path, timeout)
	if err != nil {
		return err
	}
	defer l.Release()
	return fn()
}
