//go:build !windows

package live

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// psStart is how ps -o lstart= prints a start time, in C locale
const psStart = "Mon Jan _2 15:04:05 2006"

// probeProc asks ps about one pid. a pid ps does not know is not alive; ps that
// cannot be run is an error, since then nothing is known.
func probeProc(ctx context.Context, pid int) (Proc, error) {
	cmd := exec.CommandContext(ctx, "ps", "-ww", "-o", "lstart=,ppid=,command=", "-p", strconv.Itoa(pid))
	cmd.Env = append(cmd.Environ(), "LC_ALL=C")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		// ps exits 1 with nothing to say for a pid it does not know
		if _, exited := err.(*exec.ExitError); exited && strings.TrimSpace(out.String()) == "" && strings.TrimSpace(errb.String()) == "" {
			return Proc{}, nil
		}
		return Proc{}, err
	}
	return parsePS(pid, out.String())
}

// parsePS reads "Thu Oct 23 15:05:12 2026 <ppid> <command>"
func parsePS(pid int, out string) (Proc, error) {
	line, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	f := strings.Fields(line)
	if len(f) < 7 {
		return Proc{}, fmt.Errorf("ps answered about pid %d in a form clalt does not know", pid)
	}
	start, err := time.ParseInLocation(psStart, strings.Join(f[:5], " "), time.Local)
	ppid, err2 := strconv.Atoi(f[5])
	if err != nil || err2 != nil {
		return Proc{}, fmt.Errorf("ps answered about pid %d in a form clalt does not know", pid)
	}
	cmd := strings.Join(f[6:], " ")
	// a process that ended and waits to be collected
	if strings.Contains(cmd, "<defunct>") {
		return Proc{}, nil
	}
	return Proc{Alive: true, Start: start, PPID: ppid, Command: cmd}, nil
}

// signalPID asks the process to end, as a ^C from the terminal would not
func signalPID(pid int) error { return syscall.Kill(pid, syscall.SIGTERM) }
