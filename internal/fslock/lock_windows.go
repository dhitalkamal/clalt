//go:build windows

package fslock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// errWouldBlock signals that another handle currently holds the lock;
// Acquire retries until timeout on this specific error.
var errWouldBlock = errors.New("lock held elsewhere")

// tryLock takes a non-blocking exclusive lock on f's whole extent.
func tryLock(f *os.File) error {
	ol := new(windows.Overlapped)
	err := windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		^uint32(0), // lock the whole file: max bytesLow/bytesHigh
		^uint32(0),
		ol,
	)
	if err == nil {
		return nil
	}
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return errWouldBlock
	}
	return err
}

// unlockFile releases the lock held on f's whole extent.
func unlockFile(f *os.File) error {
	ol := new(windows.Overlapped)
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, ^uint32(0), ^uint32(0), ol)
}
