//go:build !windows

package officed

import (
	"os"
	"os/exec"
	"syscall"
)

// Spawn starts exe with args detached: its own session, no stdin, stdout and
// stderr appended to the log file (0600).
func Spawn(exe string, args []string, logPath string) (Child, error) {
	log, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return Child{}, err
	}
	defer log.Close()
	cmd := exec.Command(exe, args...)
	// nil stdin is /dev/null
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return Child{}, err
	}
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	return Child{PID: cmd.Process.Pid, Exited: exited}, nil
}
