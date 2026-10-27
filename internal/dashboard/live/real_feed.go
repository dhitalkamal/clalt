package live

import (
	"context"
	"math"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
)

// FeedFiles is the Feed on the files claude's hooks and statusline write.
type FeedFiles struct {
	Store feed.Store
	Now   func() time.Time
}

var _ Feed = FeedFiles{}

func (f FeedFiles) now() time.Time {
	if f.Now != nil {
		return f.Now()
	}
	return time.Now()
}

// Limits are the account's two windows as the feed last merged them. a window
// the feed lacks is Unknown, and an account with neither has no reading. a
// window whose reset time passed is kept as it is: mapLimits shows it as reset
// and ReadAt says how old the reading is.
func (f FeedFiles) Limits(_ context.Context, accountID string) (Limits, bool, error) {
	l, ok := f.Store.Limits(accountID)
	if !ok || (l.FiveHour == nil && l.SevenDay == nil) {
		return Limits{}, false, nil
	}
	return Limits{Five: window(l.FiveHour), Seven: window(l.SevenDay), ObservedAt: l.ObservedAt}, true, nil
}

func window(w *feed.Window) Window {
	if w == nil {
		return Window{Unknown: true}
	}
	return Window{Percent: int(math.Round(w.UsedPct)), ResetsAt: w.ResetsAt}
}

// Readings has no history: the feed keeps the latest reading only, so the
// limits chart says there are no readings yet.
func (FeedFiles) Readings(context.Context, string, time.Time) ([]Reading, error) { return nil, nil }

// Installed says whether the hooks of the account have reported: clalt run
// installs them per session, so what shows they work is a reading or a session
// state in the feed.
func (f FeedFiles) Installed(accountID string) bool {
	if config.CheckAccountID(accountID) != nil {
		return false
	}
	if _, ok := f.Store.Limits(accountID); ok {
		return true
	}
	all, err := f.Store.Sessions()
	if err != nil {
		return false
	}
	for _, s := range all {
		if s.Account == accountID {
			return true
		}
	}
	return false
}
