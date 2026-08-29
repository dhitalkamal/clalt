//go:build !windows

package profile

import "syscall"

// alive reports whether a process exists. Signal 0 performs error checking
// without sending a signal; EPERM means it exists but is not ours.
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
