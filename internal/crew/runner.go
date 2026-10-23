package crew

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"time"
)

// Spec is one process to run.
type Spec struct {
	Bin  string
	Args []string
	// Env is the whole environment of the process. nil means the caller's own.
	Env []string
	Dir string
}

// Runner is how clalt runs programs. everything the package starts or asks about
// goes through it, so that a test can stand in for kirocrew and for ps.
type Runner interface {
	// Output runs spec to its end. a program that exits with a code other than
	// 0 is an error that has ExitCode, and what it printed comes back with it.
	Output(ctx context.Context, spec Spec) (stdout, stderr []byte, err error)
	// Start launches spec in a session of its own, so that it outlives clalt and
	// the terminal, with no input and its stdout and stderr appended to the file
	// at logPath. it does not wait for it.
	Start(spec Spec, logPath string) (pid int, err error)
}

// ExecRunner is the Runner that runs real programs.
type ExecRunner struct{}

// outputGrace is how long Output waits for a program's pipes after the context
// ended and the program was killed: a child that inherited them can hold them.
const outputGrace = 2 * time.Second

// Output runs the program and waits for it.
func (ExecRunner) Output(ctx context.Context, spec Spec) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, spec.Bin, spec.Args...)
	cmd.Env, cmd.Dir = spec.Env, spec.Dir
	cmd.WaitDelay = outputGrace
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// Start launches the program detached and returns its pid.
func (ExecRunner) Start(spec Spec, logPath string) (int, error) {
	log, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return 0, err
	}
	defer log.Close()
	// a log from an earlier version may have been made with looser rights
	_ = log.Chmod(0o600)

	// not CommandContext: nothing here may end the program but its own stop
	cmd := exec.Command(spec.Bin, spec.Args...)
	cmd.Env, cmd.Dir = spec.Env, spec.Dir
	cmd.Stdout, cmd.Stderr = log, log
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	// whoever ends it, a program that nobody waits for stays a zombie, and a
	// zombie still answers signal 0 as if it were running
	go func() { _ = cmd.Wait() }()
	return cmd.Process.Pid, nil
}
