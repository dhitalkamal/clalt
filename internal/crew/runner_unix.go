//go:build !windows

package crew

import (
	"os/exec"
	"syscall"
)

// detach puts the program in a session of its own: a Ctrl-C or a closed
// terminal reaches the foreground group of the terminal, and the gateway is
// not in it.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
