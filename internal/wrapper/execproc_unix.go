//go:build !windows

package wrapper

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// terminate sends SIGTERM. the docs say claude answers it by ending its bash
// commands, running its SessionEnd hooks and exiting with 143, starting nothing
// new. SIGINT is the interrupt key and takes a second one to exit, and SIGHUP
// says the terminal is gone. the docs describe this for `claude -p`: that an
// interactive claude also puts the terminal back is not documented, which is why
// the wrapper restores it itself, see Terminal.
func terminate(p *os.Process) error { return p.Signal(syscall.SIGTERM) }

// terminated says whether claude ended the way SIGTERM makes it end: by the
// signal, or with 143, or killed. a clean exit, or any other code, is the user.
func terminated(err error) bool {
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return false
	}
	if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return ws.Signal() == syscall.SIGTERM || ws.Signal() == syscall.SIGKILL
	}
	return ee.ExitCode() == 143
}
