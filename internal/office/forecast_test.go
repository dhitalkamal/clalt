package office

import (
	"testing"
	"time"
)

type fcLimits map[string]Limits

func (f fcLimits) Limits(id string) (Limits, bool) { l, ok := f[id]; return l, ok }

type fcSessions []Session

func (f fcSessions) Sessions() ([]Session, error) { return f, nil }

type fcAccounts []Account

func (f fcAccounts) Accounts() ([]Account, error) { return f, nil }

// a five hour window with the given use and reset time
func win5(pct float64, resets time.Time) *Window { return &Window{UsedPct: pct, ResetsAt: resets} }

func TestForecastHitsTheLimitAtTheAveragePace(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	resets := now.Add(2 * time.Hour) // window started 3h ago
	// 75 percent in 3h is 25 per hour: 25 left is 1h
	l := Limits{FiveHour: win5(75, resets), ReadAt: now}
	f := forecastOf(l, true, now)
	if f.FiveHourAt == nil || !f.FiveHourAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("fiveHourAt = %v, want in 1h", f.FiveHourAt)
	}
	if f.PacePctPerHour != 25 {
		t.Errorf("pace = %v, want 25", f.PacePctPerHour)
	}
}

func TestForecastIsNullWhenThereIsNothingToForecast(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	resets := now.Add(2 * time.Hour)
	cases := map[string]struct {
		l    Limits
		live bool
	}{
		"no live session":     {Limits{FiveHour: win5(75, resets), ReadAt: now}, false},
		"no five hour window": {Limits{ReadAt: now}, true},
		"never read":          {Limits{FiveHour: win5(75, resets)}, true},
		"stale reading":       {Limits{FiveHour: win5(75, resets), ReadAt: now.Add(-time.Hour)}, true},
		"nothing used":        {Limits{FiveHour: win5(0, resets), ReadAt: now}, true},
		"will not fill":       {Limits{FiveHour: win5(10, resets), ReadAt: now}, true},
		"window just began":   {Limits{FiveHour: win5(3, now.Add(5*time.Hour-time.Minute)), ReadAt: now}, true},
	}
	for name, c := range cases {
		if f := forecastOf(c.l, c.live, now); f.FiveHourAt != nil || f.PacePctPerHour != 0 {
			t.Errorf("%s: forecast %+v, want empty", name, f)
		}
	}
}

func TestForecastNeverPointsIntoThePast(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	// read 10 minutes ago at 99 percent after 3h: it filled since
	l := Limits{FiveHour: win5(99.5, now.Add(2*time.Hour)), ReadAt: now.Add(-10 * time.Minute)}
	f := forecastOf(l, true, now)
	if f.FiveHourAt == nil || f.FiveHourAt.Before(now) {
		t.Errorf("fiveHourAt = %v, want now at the earliest", f.FiveHourAt)
	}
}

func TestBuildFillsForecastAndActions(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	src := Sources{
		Accounts: fcAccounts{{ID: "work", Label: "WORK"}, {ID: "idle", Label: "IDLE"}},
		Limits: fcLimits{
			"work": {FiveHour: win5(75, now.Add(2*time.Hour)), ReadAt: now},
			"idle": {FiveHour: win5(75, now.Add(2*time.Hour)), ReadAt: now},
		},
		Sessions: fcSessions{
			{Handle: "0123456789abcdef", Account: "work", Cwd: "/p/x", State: "busy", StartedAt: now, Can: ActionsOut{Move: true, Focus: true, Stop: true}},
			{Handle: "ext-abcdef012345", Account: "work", Cwd: "/p/y", State: "busy", StartedAt: now, Can: ActionsOut{Move: true, Focus: true, Stop: true}},
		},
		Now: func() time.Time { return now },
	}
	snap, err := Build(src)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]AccountOut{}
	for _, a := range snap.Accounts {
		by[a.ID] = a
	}
	if by["work"].Forecast.FiveHourAt == nil {
		t.Error("work has a live session and a pace, want a forecast")
	}
	if by["idle"].Forecast.FiveHourAt != nil {
		t.Error("idle has no session, want no forecast")
	}
	run, ext := snap.Sessions[0], snap.Sessions[1]
	if run.Handle != "0123456789abcdef" {
		run, ext = ext, run
	}
	if !run.Actions.Move || !run.Actions.Focus || !run.Actions.Stop || run.Actions.Why != "" {
		t.Errorf("run actions = %+v", run.Actions)
	}
	if ext.Actions.Move || ext.Actions.Focus || ext.Actions.Stop || ext.Actions.Why != "started outside clalt: use clalt run to control it" {
		t.Errorf("ext actions = %+v", ext.Actions)
	}
}
