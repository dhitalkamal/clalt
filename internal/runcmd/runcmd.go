// Package runcmd runs the outside programs clalt drives (kitty, tmux, osascript,
// notify-send) behind one small interface, so code that drives them is tested
// without any of them installed or started.
package runcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runner runs an outside program to the end.
type Runner interface {
	// Run starts name with args and waits for it. the program gets no input. a
	// program that ran and exited with a non-zero code is a *Failure and still
	// returns what it wrote to stdout. any other error means it did not run or
	// did not finish.
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
	// Has reports whether name can be found.
	Has(name string) bool
}

// Failure is a program that ran and exited with a non-zero code.
type Failure struct {
	Name   string
	Code   int
	Stderr string
}

func (f *Failure) Error() string {
	if line := firstLine(f.Stderr); line != "" {
		return fmt.Sprintf("%s exited %d: %s", f.Name, f.Code, line)
	}
	return fmt.Sprintf("%s exited %d", f.Name, f.Code)
}

func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			if len(line) > 200 {
				line = strings.ToValidUTF8(line[:200], "")
			}
			return line
		}
	}
	return ""
}

// stderrCap bounds what a Failure keeps of stderr.
const stderrCap = 4096

// Exec runs programs for real. Timeout bounds a call whose context has no
// deadline of its own, 15 seconds when zero.
type Exec struct{ Timeout time.Duration }

// Run implements Runner.
func (e Exec) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if _, ok := ctx.Deadline(); !ok {
		timeout := e.Timeout
		if timeout <= 0 {
			timeout = 15 * time.Second
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	// a child that outlives the program and holds the pipes must not hold us
	cmd.WaitDelay = time.Second

	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil, errors.Is(err, exec.ErrWaitDelay):
		return stdout.Bytes(), nil
	case ctx.Err() != nil:
		return nil, ended(name, ctx.Err())
	case errors.As(err, &exit):
		text := stderr.String()
		if len(text) > stderrCap {
			text = strings.ToValidUTF8(text[:stderrCap], "")
		}
		return stdout.Bytes(), &Failure{Name: name, Code: exit.ExitCode(), Stderr: text}
	}
	return nil, fmt.Errorf("run %s: %w", name, err)
}

func ended(name string, why error) error {
	if errors.Is(why, context.DeadlineExceeded) {
		return fmt.Errorf("%s did not answer in time: %w", name, why)
	}
	return fmt.Errorf("%s was stopped: %w", name, why)
}

// Has implements Runner.
func (Exec) Has(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
