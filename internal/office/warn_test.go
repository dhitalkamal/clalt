package office

import (
	"context"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/notify"
)

type recWarner struct{ got []notify.LimitEvent }

func (r *recWarner) LimitSoon(_ context.Context, e notify.LimitEvent) (notify.Outcome, error) {
	r.got = append(r.got, e)
	return notify.Sent, nil
}

var warnNow = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func acct(id, label string, pct float64, full *time.Time) AccountOut {
	a := AccountOut{ID: id, Label: label, Forecast: ForecastOut{FiveHourAt: full}}
	a.Limits.FiveHour = &WindowOut{UsedPct: pct, ResetsAt: warnNow.Add(2 * time.Hour)}
	return a
}

func at(d time.Duration) *time.Time { t := warnNow.Add(d); return &t }

func TestWarnLimitsNamesTheAccountWithTheMostRoom(t *testing.T) {
	snap := Snapshot{Accounts: []AccountOut{
		acct("work", "WORK", 80, at(19*time.Minute)),
		acct("personal", "PERSONAL", 12, nil),
		acct("side", "SIDE", 40, nil),
	}}
	w := &recWarner{}
	WarnLimits(context.Background(), snap, warnNow, w, nil)
	if len(w.got) != 1 {
		t.Fatalf("%d warnings, want 1", len(w.got))
	}
	e := w.got[0]
	if e.AccountID != "work" || e.Account != "WORK" || e.Alt != "PERSONAL" || e.AltPct != 12 || e.Minutes != 19 {
		t.Errorf("event = %+v", e)
	}
	if !e.WindowEnds.Equal(warnNow.Add(2 * time.Hour)) {
		t.Errorf("window = %v", e.WindowEnds)
	}
}

func TestWarnLimitsWaitsUntilTwentyMinutesAreLeft(t *testing.T) {
	snap := Snapshot{Accounts: []AccountOut{acct("work", "WORK", 80, at(21*time.Minute)), acct("p", "P", 10, nil)}}
	w := &recWarner{}
	WarnLimits(context.Background(), snap, warnNow, w, nil)
	if len(w.got) != 0 {
		t.Errorf("warned at 21 minutes: %+v", w.got)
	}
}

func TestWarnLimitsDoesNotPointAtAnAccountThatIsRunningOutToo(t *testing.T) {
	snap := Snapshot{Accounts: []AccountOut{
		acct("a", "A", 80, at(10*time.Minute)),
		acct("b", "B", 5, at(15*time.Minute)),
	}}
	w := &recWarner{}
	WarnLimits(context.Background(), snap, warnNow, w, nil)
	for _, e := range w.got {
		if e.Alt != "" {
			t.Errorf("%s was told to move to %s, which runs out too", e.Account, e.Alt)
		}
	}
	if len(w.got) != 2 {
		t.Errorf("%d warnings, want one per account", len(w.got))
	}
}

func TestWarnLimitsOncePerWindowThroughTheRealAlerts(t *testing.T) {
	snap := Snapshot{Accounts: []AccountOut{acct("work", "WORK", 80, at(5*time.Minute)), acct("p", "P", 10, nil)}}
	var shown int
	a := &notify.Alerts{Notifier: countNotifier{&shown}, Dir: t.TempDir(), Now: func() time.Time { return warnNow }}
	for i := 0; i < 4; i++ {
		WarnLimits(context.Background(), snap, warnNow.Add(time.Duration(i)*time.Second), a, nil)
	}
	if shown != 1 {
		t.Errorf("%d notifications, want 1", shown)
	}
}

type countNotifier struct{ n *int }

func (c countNotifier) Notify(context.Context, notify.Note) error { *c.n++; return nil }

func TestWarnLimitsTreatsAResetWindowAsFreeButNoReadingAsUnknown(t *testing.T) {
	read := warnNow
	reset := AccountOut{ID: "reset", Label: "RESET"}
	reset.Limits.ReadAt = &read
	never := AccountOut{ID: "never", Label: "NEVER"}
	snap := Snapshot{Accounts: []AccountOut{acct("work", "WORK", 80, at(5*time.Minute)), never, reset}}
	w := &recWarner{}
	WarnLimits(context.Background(), snap, warnNow, w, nil)
	if len(w.got) != 1 || w.got[0].Alt != "RESET" || w.got[0].AltPct != 0 {
		t.Errorf("events = %+v", w.got)
	}
}
