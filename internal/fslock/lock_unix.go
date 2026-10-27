//go:build !windows

package fslock

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// errWouldBlock signals that another open file description currently holds
// the lock; Acquire retries until timeout on this specific error.
var errWouldBlock = errors.New("lock held elsewhere")

// tryLock takes a non-blocking exclusive flock on f.
func tryLock(f *os.File) error {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err == nil {
		return nil
	}
	if errors.Is(err, unix.EWOULDBLOCK) {
		return errWouldBlock
	}
	return err
}

// unlockFile releases the flock held on f.
func unlockFile(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}
