package claudecli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

type result struct {
	stdout, stderr []byte
	code           int
	signal         string // the signal that ended claude, empty when it exited by itself
}

// ended says how claude ended, for messages: a signal is named, since its exit
// code would only read -1.
func (r result) ended() string {
	if r.signal != "" {
		return "ended by signal: " + r.signal
	}
	return fmt.Sprintf("exit %d", r.code)
}

// cancelGrace is how long a cancelled claude gets to finish before it is killed.
// the point is a token refresh in flight: claude 2.1.301 fixed folders being
// signed out when `claude auth status` exited in the middle of one. note that
// claude 2.1.303's `auth status` has no SIGINT handler, so an interrupt ends it
// at once and the grace only helps a claude that catches the signal. what
// protects a refresh in practice is that callers do not cancel auth calls early:
// Status and Logout get a long timeout of their own when the caller sets none.
var cancelGrace = 30 * time.Second

// goos is a seam so a test can exercise the Windows rule anywhere.
var goos = runtime.GOOS

// CancelError is what a call returns when its context ended before claude did.
// Killed is false when claude stopped by itself after the interrupt, and true
// when it did not and was killed once the grace period was over. Err is the
// context's error, so errors.Is(err, context.Canceled) and DeadlineExceeded
// still work.
type CancelError struct {
	Killed bool
	Err    error
}

// Error says how the call ended.
func (e *CancelError) Error() string {
	if e.Killed {
		return "killed after the grace period: " + e.Err.Error()
	}
	return "interrupted: " + e.Err.Error()
}

// Unwrap gives the context's error.
func (e *CancelError) Unwrap() error { return e.Err }

// cancelState remembers when the context ended and claude was asked to stop.
type cancelState struct{ at atomic.Int64 }

func (c *cancelState) mark() { c.at.CompareAndSwap(0, time.Now().UnixNano()) }

func (c *cancelState) sent() bool { return c.at.Load() != 0 }

func (c *cancelState) since() time.Duration { return time.Since(time.Unix(0, c.at.Load())) }

// command builds a claude command bound to ctx, with the given environment.
// cancelling asks claude to stop with an interrupt instead of killing it, and
// the kill only comes once the grace period is over. windows cannot send an
// interrupt to another process, so there nothing is sent and the grace period
// simply runs before the kill.
func command(ctx context.Context, env []string, args ...string) (*exec.Cmd, *cancelState, error) {
	bin, err := Binary()
	if err != nil {
		return nil, nil, err
	}
	st := &cancelState{}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env
	cmd.Cancel = func() error {
		if goos == "windows" {
			st.mark()
			return nil
		}
		err := cmd.Process.Signal(os.Interrupt)
		if err == nil {
			st.mark()
		}
		return err
	}
	cmd.WaitDelay = cancelGrace
	return cmd, st, nil
}

// run waits for claude and captures its output. a non-zero exit is data, not an
// error, because callers give each code its own meaning. an error means claude
// could not run, or ctx ended while it was running (a *CancelError). it is for
// the calls clalt makes on its own, which read no terminal, so they start in a
// group of their own, see ownGroup. the sign-in and the session do not: they
// are built by command and run by the caller, in the foreground.
func run(ctx context.Context, env []string, args ...string) (result, error) {
	cmd, st, err := command(ctx, env, args...)
	if err != nil {
		return result{}, err
	}
	ownGroup(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	return finish(runErr, ctx.Err(), st, stdout.Bytes(), stderr.Bytes())
}

// finish turns what cmd.Run returned into a result or an error. a result that
// claude produced on its own is kept even if the context ended a moment later;
// only a call that was interrupted or killed because of the context is a
// cancel. an exit that came after the interrupt, even a clean one, is a cancel
// too, since exec reports it as one and the output cannot be trusted to be whole.
func finish(runErr, ctxErr error, st *cancelState, stdout, stderr []byte) (result, error) {
	res := result{stdout: stdout, stderr: stderr}
	var exit *exec.ExitError
	if errors.As(runErr, &exit) {
		res.code = exit.ExitCode()
		if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			res.signal = ws.Signal().String()
		}
	}
	switch {
	case runErr == nil, errors.Is(runErr, exec.ErrWaitDelay):
		// pipes held open by a child after a clean exit are not a failure: the
		// caller parses what was read and fails if it is not whole
		return res, nil
	case st.sent():
		if ctxErr == nil {
			ctxErr = context.Canceled
		}
		killed := st.since() >= cancelGrace || res.signal == syscall.SIGKILL.String()
		return result{}, &CancelError{Killed: killed, Err: ctxErr}
	case exit != nil:
		return res, nil
	}
	return result{}, runErr
}

// failure describes a command that did not do its job, with the first
// meaningful line of its stderr (and nothing more: stdout may hold an account's
// details).
func (r result) failure(what, problem string) error {
	if line := meaningfulLine(r.stderr); line != "" {
		return fmt.Errorf("claude %s: %s: %s", what, problem, line)
	}
	return fmt.Errorf("claude %s: %s", what, problem)
}

// meaningfulLine is the first line of b that says something: blank lines and
// the deprecation warnings that node prints before the real output are skipped.
func meaningfulLine(b []byte) string {
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !runtimeNoise(line) {
			return cut(line)
		}
	}
	return ""
}

func runtimeNoise(line string) bool {
	return strings.HasPrefix(line, "(node:") || strings.HasPrefix(line, "(Use `node") ||
		strings.Contains(line, "DeprecationWarning")
}

func firstLine(b []byte) string {
	line, _, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
	return cut(strings.TrimSpace(line))
}

// cut keeps a line short enough to put in an error, without splitting a
// multi-byte character.
func cut(line string) string {
	if len(line) > 200 {
		line = strings.ToValidUTF8(line[:200], "")
	}
	return line
}
