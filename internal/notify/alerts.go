package notify

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/fslock"
)

// Outcome says what NeedsYou did.
type Outcome int

// the outcomes of NeedsYou. the zero Outcome comes with an error.
const (
	Sent Outcome = iota + 1
	Disabled
	AlreadyNotified
	NoNotifier
)

// markerLife is how long a waiting marker holds back more notifications, so a
// lost EndWait cannot silence a session for good.
const markerLife = 6 * time.Hour

// ErrBadHandle is returned for a handle that is not safe as a file name.
var ErrBadHandle = errors.New("bad session handle")

// Event is a session that is waiting for the user.
type Event struct {
	Handle  string
	Account string
	Reason  string
	Cwd     string
}

// Alerts sends needs-you notifications at most once per waiting session.
type Alerts struct {
	Notifier Notifier
	Dir      string
	Now      func() time.Time
}

func validHandle(h string) bool {
	if h == "" || len(h) > 64 || h[0] == '.' {
		return false
	}
	for _, c := range h {
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '-' || c == '_'
		if !ok {
			return false
		}
	}
	return true
}

// NeedsYou notifies about e unless the setting is off, a notification for this
// session is already out, or nothing on this machine can show one.
func (a *Alerts) NeedsYou(ctx context.Context, e Event) (Outcome, error) {
	if !validHandle(e.Handle) {
		return 0, ErrBadHandle
	}
	// a setting that cannot be read means on, see Enabled
	if on, _ := Enabled(a.Dir); !on {
		return Disabled, nil
	}
	if a.Notifier == nil {
		return NoNotifier, nil
	}
	made, err := a.claim(e.Handle)
	if err != nil {
		return 0, err
	}
	if !made {
		return AlreadyNotified, nil
	}
	if err := a.Notifier.Notify(ctx, note(e)); err != nil {
		// let the next call try again
		os.Remove(a.marker(e.Handle))
		return 0, err
	}
	return Sent, nil
}

// EndWait forgets that handle was waiting.
func (a *Alerts) EndWait(handle string) error {
	if !validHandle(handle) {
		return ErrBadHandle
	}
	if err := os.Remove(a.marker(handle)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (a *Alerts) marker(handle string) string { return filepath.Join(a.Dir, "waiting", handle) }

// claim makes the marker file. false means a fresh one was already there. the
// exclusive create is the atomic check and set.
func (a *Alerts) claim(handle string) (bool, error) {
	p := a.marker(handle)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return false, err
	}
	for try := 0; try < 2; try++ {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			return true, f.Close()
		}
		if !errors.Is(err, fs.ErrExist) {
			return false, err
		}
		fi, serr := os.Stat(p)
		if errors.Is(serr, fs.ErrNotExist) {
			continue
		}
		if serr != nil {
			return false, serr
		}
		now := time.Now
		if a.Now != nil {
			now = a.Now
		}
		if now().Sub(fi.ModTime()) <= markerLife {
			return false, nil
		}
		return a.replaceStale(p, now)
	}
	return false, nil
}

// replaceStale makes the marker again when it is still stale once the lock of
// this handle is held. of several hooks that found the same stale marker, the
// first replaces it and the others find a fresh one.
func (a *Alerts) replaceStale(p string, now func() time.Time) (bool, error) {
	lock, err := fslock.Acquire(filepath.Join(filepath.Dir(p), ".lock-"+filepath.Base(p)), 5*time.Second)
	if err != nil {
		return false, err
	}
	defer lock.Release()
	if fi, err := os.Stat(p); err == nil && now().Sub(fi.ModTime()) <= markerLife {
		return false, nil
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return false, nil
	}
	return true, f.Close()
}

func note(e Event) Note {
	text := strings.TrimSpace(e.Reason)
	if text == "" {
		text = "waiting for you"
	}
	if e.Cwd != "" {
		if last := path.Base(filepath.ToSlash(filepath.Clean(e.Cwd))); last != "." && last != "/" {
			text += " (" + last + ")"
		}
	}
	return Note{Title: e.Account + " needs you", Text: text}
}
