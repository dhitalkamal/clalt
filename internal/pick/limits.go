// Package pick chooses an account for a folder: which of the allowed accounts
// has the most room left in its usage limits. it is used to pick an account for
// `clalt run` and to find the account an auto-switch should move a session to.
package pick

import (
	"math"
	"time"
)

// DefaultMaxAge is how old limit data may be and still count as recent. a
// percentage only moves with use, and use through claude is reported as it
// happens, so an old figure is still a lower bound until its window resets. what
// it cannot see is use outside claude code, such as claude.ai in the browser,
// which draws on the same limits.
const DefaultMaxAge = 24 * time.Hour

// Window is how much of one usage limit window is used.
type Window struct {
	Used     float64 // percent, 0 to 100
	ResetsAt time.Time
	Known    bool
}

// Limits are the limits of one account as the feed last saw them.
type Limits struct {
	FiveHour, SevenDay Window
	ObservedAt         time.Time
}

// used is the percent of the window in use at now. a window that has reset since
// the figure was taken is empty again, and one with no reset time is taken as it
// was reported.
func (w Window) used(now time.Time) (float64, bool) {
	if !w.Known || math.IsNaN(w.Used) {
		return 0, false
	}
	if !w.ResetsAt.IsZero() && !w.ResetsAt.After(now) {
		return 0, true
	}
	return math.Min(100, math.Max(0, w.Used)), true
}

// Room is the percent of its limits an account has left: 100 minus the higher
// of the used percentages of the windows that are known. it is false when none
// is.
func (l Limits) Room(now time.Time) (float64, bool) {
	var used float64
	var known bool
	for _, w := range []Window{l.FiveHour, l.SevenDay} {
		if u, ok := w.used(now); ok {
			used, known = math.Max(used, u), true
		}
	}
	if !known {
		return 0, false
	}
	return 100 - used, true
}

// Recent says whether the data was seen within maxAge of now. data that was never
// seen is not recent, and a time a little ahead of this clock is.
func (l Limits) Recent(now time.Time, maxAge time.Duration) bool {
	return !l.ObservedAt.IsZero() && now.Sub(l.ObservedAt) <= maxAge
}

// FiveHourUsed is the percent of the five-hour window in use at now, which is
// what auto-switch watches. false when that window is not known.
func (l Limits) FiveHourUsed(now time.Time) (float64, bool) { return l.FiveHour.used(now) }
