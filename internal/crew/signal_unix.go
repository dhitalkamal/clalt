//go:build !windows

package crew

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

// terminate asks a process to stop, which is what stopping a gateway is: it
// shuts down by itself, and clalt waits. a gateway leads a session of its own,
// so its group is the gateway and what it started (the claude adapters), and
// the signal goes to the group when the pid leads one, so that nothing is
// left running behind it. a process that leads no group of its own gets the
// signal alone. a pid of 1 or below would be init, or a whole process group,
// or every process there is, so it is refused whatever the caller believes.
func terminate(pid int) error {
	if pid <= 1 {
		return fmt.Errorf("refusing to signal pid %d", pid)
	}
	if pgid, err := unix.Getpgid(pid); err == nil && pgid == pid {
		return syscall.Kill(-pid, syscall.SIGTERM)
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}
