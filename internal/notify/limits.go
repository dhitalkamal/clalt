package notify

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// LimitEvent is an account that is about to reach its 5h limit. WindowEnds is
// when the window resets and names it, so the warning is sent once per window.
// Alt is the account with the most room, empty when there is none.
type LimitEvent struct {
	AccountID  string
	Account    string
	Minutes    int
	WindowEnds time.Time
	Alt        string
	AltPct     int
}

// limitMarkerLife is how long a marker is kept, long enough that a window can
// never come back, short enough that the folder stays small.
const limitMarkerLife = 24 * time.Hour

// LimitSoon warns about e unless the setting is off or this account already got
// its warning for this window. the marker is the exclusive create of a file named
// by account and window.
func (a *Alerts) LimitSoon(ctx context.Context, e LimitEvent) (Outcome, error) {
	if !validHandle(e.AccountID) {
		return 0, ErrBadHandle
	}
	if on, _ := Enabled(a.Dir); !on {
		return Disabled, nil
	}
	if a.Notifier == nil {
		return NoNotifier, nil
	}
	now := time.Now
	if a.Now != nil {
		now = a.Now
	}
	dir := filepath.Join(a.Dir, "limits")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return 0, err
	}
	sweepLimitMarkers(dir, now())
	p := filepath.Join(dir, e.AccountID+"-"+strconv.FormatInt(e.WindowEnds.Unix(), 10))
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return AlreadyNotified, nil
	}
	if err != nil {
		return 0, err
	}
	_ = f.Close()
	if err := a.Notifier.Notify(ctx, limitNote(e)); err != nil {
		_ = os.Remove(p)
		return 0, err
	}
	return Sent, nil
}

// sweepLimitMarkers removes the markers of windows that are long over.
func sweepLimitMarkers(dir string, now time.Time) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, fi := range files {
		if info, err := fi.Info(); err == nil && now.Sub(info.ModTime()) > limitMarkerLife {
			_ = os.Remove(filepath.Join(dir, fi.Name()))
		}
	}
}

func limitNote(e LimitEvent) Note {
	text := fmt.Sprintf("%s hits its 5h limit in about %d min.", e.Account, e.Minutes)
	if e.Alt != "" {
		text += fmt.Sprintf(" Move sessions to %s (%d%%).", e.Alt, e.AltPct)
	}
	return Note{Title: e.Account + " is near its limit", Text: text}
}
