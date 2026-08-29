package scheduler

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// taskschedSched is the Windows backend, driving the built-in schtasks tool to
// register a per-minute scheduled task.
type taskschedSched struct{}

func (taskschedSched) Name() string { return "schtasks" }

func (taskschedSched) Install(j Job) error {
	args := schtasksCreateArgs(j.Label, j.Program, j.Args, intervalMinutes(j.Interval))
	if err := runCmd("schtasks", args...); err != nil {
		return fmt.Errorf("schtasks create: %w", err)
	}
	if j.RunAtLoad {
		_ = runCmd("schtasks", "/Run", "/TN", j.Label)
	}
	return nil
}

func (taskschedSched) Uninstall(label string) error {
	// /F makes delete non-interactive; a missing task returns non-zero, which we
	// treat as already-gone rather than an error.
	_ = runCmd("schtasks", "/Delete", "/TN", label, "/F")
	return nil
}

func (taskschedSched) Status(label string) (bool, error) {
	err := exec.Command("schtasks", "/Query", "/TN", label).Run()
	return err == nil, nil
}

func (taskschedSched) Location(label string) string {
	return `Task Scheduler task "` + label + `"`
}

func (taskschedSched) Plan(j Job) string {
	args := schtasksCreateArgs(j.Label, j.Program, j.Args, intervalMinutes(j.Interval))
	return "Task Scheduler: run schtasks " + strings.Join(args, " ")
}

// schtasksCreateArgs builds the schtasks /Create argument list for a per-minute
// task. The /TR value is a single string carrying the quoted program and its
// args, since schtasks takes the whole command as one token.
func schtasksCreateArgs(label, program string, args []string, minutes int) []string {
	tr := `"` + program + `"`
	if len(args) > 0 {
		tr += " " + strings.Join(args, " ")
	}
	return []string{
		"/Create",
		"/TN", label,
		"/TR", tr,
		"/SC", "MINUTE",
		"/MO", fmt.Sprintf("%d", minutes),
		"/F",
	}
}

// runCmd runs a command and returns its error; shared by the exec-based
// backends. Kept here (not launchd.go) so it exists on every build.
func runCmd(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

// homeDir resolves the user home dir for backends that need it.
func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	return os.Getenv("HOME")
}
