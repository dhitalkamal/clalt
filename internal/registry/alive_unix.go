//go:build !windows

package registry

import "syscall"

// pidAlive reports whether a process exists. signal 0 only checks; EPERM means
// it exists but is not ours.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
