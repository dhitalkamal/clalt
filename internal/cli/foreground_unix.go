//go:build !windows

package cli

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// inForegroundOfTerminal says whether clalt is in the foreground process group
// of its controlling terminal, which is when that terminal's Ctrl-C reaches every
// process of the group, claude included. no controlling terminal, or one that
// cannot be asked, is no.
func inForegroundOfTerminal() bool {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return false
	}
	defer tty.Close()
	group, err := unix.IoctlGetInt(int(tty.Fd()), unix.TIOCGPGRP)
	return err == nil && group == syscall.Getpgrp()
}
