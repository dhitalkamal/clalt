//go:build !windows

package claudecli

import (
	"os/exec"
	"syscall"
)

// ownGroup starts cmd in a process group of its own. a Ctrl-C at the terminal is
// a SIGINT to the whole foreground group, and the calls clalt makes by itself
// (the status, the sign-out, the version) are short and read no terminal. real
// claude has no SIGINT handler, so one that got the interrupt in the middle of a
// token refresh would end there and could sign the folder out. in a group of its
// own it finishes while clalt ends.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
