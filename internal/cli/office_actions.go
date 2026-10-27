package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dhitalkamal/clalt/internal/office"
	"github.com/dhitalkamal/clalt/internal/terminal"
	"github.com/dhitalkamal/clalt/internal/wrapper"
)

// officeActions are the session actions of the office page. each one goes
// through the code the commands use: a move request for the wrapper, a stop
// request for the wrapper, the terminal driver. nothing here signals a process.
type officeActions struct{ app *App }

func (a *App) officeActions() officeActions { return officeActions{app: a} }

// refused turns what a command would have printed into a refusal for the page.
func refused(err error) error {
	if err == nil {
		return nil
	}
	return &office.RefusedError{Reason: clean(err.Error())}
}

// Move asks the wrapper of the session to move it to the account at its next
// pause. the handle must be the whole handle of a session started by clalt run.
func (o officeActions) Move(handle, accountID string) error {
	_, err := o.app.requestMove(handle, accountID, true)
	return refused(err)
}

// Focus brings the terminal of the session to the front, where kitty or tmux
// can do that.
func (o officeActions) Focus(ctx context.Context, handle string) error {
	d := o.app.deps()
	running, err := d.sessions()
	if err != nil {
		return refused(fmt.Errorf("could not read the running sessions: %w", err))
	}
	var hit *terminal.Session
	for i := range running {
		if running[i].Handle == handle {
			hit = &running[i]
		}
	}
	if hit == nil {
		return refused(fmt.Errorf("no running session %s", clean(handle)))
	}
	_, err = terminal.Driver{Run: d.runner, Env: d.env}.Focus(ctx, hit.Location)
	var cannot *terminal.CannotFocus
	if errors.As(err, &cannot) {
		return refused(cannot)
	}
	return refused(err)
}

// Stop leaves a stop request for the wrapper of the session, which ends claude
// cleanly at the next pause. only a record of the registry has a wrapper.
func (o officeActions) Stop(handle string) error {
	if !moveSupported {
		return refused(errors.New("stopping a session is not supported on windows: claude cannot be asked to end cleanly there"))
	}
	reg, err := openRegistry()
	if err != nil {
		return refused(err)
	}
	recs, err := reg.List()
	if err != nil {
		return refused(err)
	}
	rec, err := matchRecord(recs, handle, true)
	if err != nil {
		return refused(err)
	}
	if feed := openFeed(); feed != nil {
		if info, ok := feed.Session(rec.Handle); ok && info.State == wrapper.Ended {
			return refused(errors.New("the session has ended"))
		}
	}
	return refused(wrapper.WriteStop(reg.Dir(), rec.Handle, time.Now()))
}
