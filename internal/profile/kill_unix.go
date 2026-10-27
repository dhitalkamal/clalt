//go:build !windows

package profile

import "syscall"

// killProcess sends SIGTERM so the session shuts down cleanly. A non-positive pid
// or a process that is already gone is not fatal here; Kill still prunes the
// record.
func killProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}
