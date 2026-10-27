package office

import (
	"context"
	"math"
	"time"

	"github.com/dhitalkamal/clalt/internal/notify"
)

// warnWithin is how close to the limit an account must be to get a warning.
const warnWithin = 20 * time.Minute

// LimitWarner sends a warning once per account and window. notify.Alerts is
// the real one.
type LimitWarner interface {
	LimitSoon(ctx context.Context, e notify.LimitEvent) (notify.Outcome, error)
}

// roomOf is the account with the most room in the 5h window that is not
// excluded and is not itself about to run out. an account with no window in
// the snapshot has had it reset, so all of it is free, unless nothing was
// ever read for the account, which is unknown and never offered.
func roomOf(accts []AccountOut, exclude string) (AccountOut, float64, bool) {
	var best AccountOut
	bestPct, found := 0.0, false
	for _, a := range accts {
		if a.ID == exclude || a.Forecast.FiveHourAt != nil {
			continue
		}
		pct := 0.0
		if a.Limits.FiveHour != nil {
			pct = a.Limits.FiveHour.UsedPct
		} else if a.Limits.ReadAt == nil {
			continue
		}
		if !found || pct < bestPct {
			best, bestPct, found = a, pct, true
		}
	}
	return best, bestPct, found
}

// WarnLimits asks w to warn about every account whose forecast is within 20
// minutes. it is called with each snapshot, the once per window part is w's.
// a failed warning is logged and tried again with the next snapshot.
func WarnLimits(ctx context.Context, snap Snapshot, now time.Time, w LimitWarner, logf func(string, ...any)) {
	for _, a := range snap.Accounts {
		at := a.Forecast.FiveHourAt
		if at == nil || a.Limits.FiveHour == nil || at.Sub(now) > warnWithin {
			continue
		}
		e := notify.LimitEvent{
			AccountID: a.ID, Account: a.Label, WindowEnds: a.Limits.FiveHour.ResetsAt,
			Minutes: max(1, int(math.Ceil(at.Sub(now).Minutes()))),
		}
		if alt, pct, ok := roomOf(snap.Accounts, a.ID); ok {
			e.Alt, e.AltPct = alt.Label, int(math.Round(pct))
		}
		if _, err := w.LimitSoon(ctx, e); err != nil && logf != nil {
			logf("limit warning for %s failed: %v", a.ID, err)
		}
	}
}
