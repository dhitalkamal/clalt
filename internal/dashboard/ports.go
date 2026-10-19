package dashboard

import (
	"context"
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
// Run and SignIn hand back a command for the dashboard to run with the
// terminal released, through tea.ExecProcess, and then resume. returning no
// command and no error is reported to the user. the other methods return an
// error that the dashboard shows inline.
type Actions interface {
	Run(ctx context.Context, id string) (*exec.Cmd, error)
	SignIn(ctx context.Context, id string) (*exec.Cmd, error)
	Rename(ctx context.Context, id, label string) error
	SetBudget(ctx context.Context, id string, daily, weekly float64) error
	SignOut(ctx context.Context, id string) error
	StopSession(ctx context.Context, id, sessionID string) error
}

// Notice is an error that only carries a message. the dashboard shows it on
// the flash line like any message instead of as a failure, and does not
// reload. the demo actions use it to say what they would have done.
type Notice string

func (n Notice) Error() string { return string(n) }
