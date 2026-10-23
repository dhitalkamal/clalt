//go:build windows

package registry

import "syscall"

// processQueryLimitedInformation is the least access that proves a process
// handle can be opened.
const processQueryLimitedInformation = 0x1000

// pidAlive reports whether a process exists by opening a handle to it.
func pidAlive(pid int) bool {
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
