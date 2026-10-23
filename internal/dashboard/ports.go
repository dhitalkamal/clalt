package dashboard

import (
	"context"
	"io"
	"os/exec"
)

// Source loads everything the dashboard shows. Load runs off the UI loop, one
// load at a time. it should stop when ctx ends, which also happens at the load
// timeout. the dashboard stops waiting then even if Load does not return, so a
// Load that ignores ctx can still be running when the next one starts.
//
// when Load returns both a snapshot and an error, the error wins: the
// dashboard shows it and keeps the last good snapshot.
type Source interface {
	Load(ctx context.Context) (Snapshot, error)
}

// Actions is what the dashboard asks the outside world to do. accounts are
// addressed by id. every method runs off the UI loop, possibly while a Load is
// running, so an implementation must be safe for concurrent use with Load and
// with itself. ctx ends when the dashboard quits.
//
// Delete removes the account for good, its folder included. the dashboard has
// asked the user to type the label first.
//
// Run and SignIn hand back a Launch for the dashboard to run with the
// terminal released, through tea.Exec, and then resume. returning no launch
// and no error is reported to the user. the other methods return an error
// that the dashboard shows inline.
type Actions interface {
	Run(ctx context.Context, id string) (Launch, error)
	SignIn(ctx context.Context, id string) (Launch, error)
	Rename(ctx context.Context, id, label string) error
	SetBudget(ctx context.Context, id string, daily, weekly float64) error
	SignOut(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	StopSession(ctx context.Context, id, sessionID string) error
}

// Launch is something the dashboard runs while the terminal belongs to it
// alone, then resumes. it is a program, or the code of an adapter that wants
// the terminal for a while: the real adapter calls the code of `clalt run`
// itself, so a refusal can stay on screen until it has been read. the four
// methods are those of tea.ExecCommand. a Launch may also have a StartError
// method: an error there means it cannot run at all, and the dashboard says so
// without releasing the screen.
type Launch interface {
	Run() error
	SetStdin(io.Reader)
	SetStdout(io.Writer)
	SetStderr(io.Writer)
}

// Command is a Launch that runs c. a stream that c already has is kept, the
// others are the terminal's. no command is no Launch.
func Command(c *exec.Cmd) Launch {
	if c == nil {
		return nil
	}
	return command{c}
}

type command struct{ c *exec.Cmd }

func (c command) Run() error { return c.c.Run() }

func (c command) SetStdin(r io.Reader) {
	if c.c.Stdin == nil {
		c.c.Stdin = r
	}
}

func (c command) SetStdout(w io.Writer) {
	if c.c.Stdout == nil {
		c.c.Stdout = w
	}
}

func (c command) SetStderr(w io.Writer) {
	if c.c.Stderr == nil {
		c.c.Stderr = w
	}
}

// StartError is why the program cannot be started, such as one that is not on
// the path. bubbletea v2.1.0 can leave its renderer stopped when an exec
// returns at once, so such a program never reaches it.
func (c command) StartError() error { return c.c.Err }

// Notice is an error that only carries a message. the dashboard shows it on
// the flash line like any message instead of as a failure, and does not
// reload. the demo actions use it to say what they would have done.
type Notice string

func (n Notice) Error() string { return string(n) }
