//go:build windows

package profile

import "syscall"

// processQueryLimitedInformation is the minimal access right needed just to
// confirm a process handle can be opened, which is enough to prove existence.
const processQueryLimitedInformation = 0x1000

// alive reports whether a process exists by trying to open a handle to it.
// OpenProcess fails when no process has that pid; a successful open (closed
// immediately) means it is present.
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	_ = syscall.CloseHandle(h)
	return true
}
