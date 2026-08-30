//go:build windows

package profile

import "syscall"

// processTerminate is the access right needed to terminate a process.
const processTerminate = 0x0001

// killProcess opens the process with terminate rights and ends it. A non-positive
// pid or an already-exited process is not fatal; Kill still prunes the record.
func killProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	h, err := syscall.OpenProcess(processTerminate, false, uint32(pid))
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(h)
	return syscall.TerminateProcess(h, 1)
}
