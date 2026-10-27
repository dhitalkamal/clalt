package summary

import (
	"testing"
	"time"
	_ "time/tzdata"
)

func f(v float64) *float64 { return &v }

func reading(ts string, five, seven *float64) Reading {
	return Reading{T: at(ts), FiveHour: five, SevenDay: seven}
}

// readingsOf serves a fixed list per account and counts how it was asked.
func readingsOf(m map[string][]Reading) func(string, time.Time) []Reading {
	return func(id string, since time.Time) []Reading {
		var out []Reading
		for _, r := range m[id] {
			if !r.T.Before(since) {
				out = append(out, r)
			}
		}
		return out
	}
}

func pctService(t *testing.T, loc *time.Location, now string, m map[string][]Reading) *Service {
	t.Helper()
	return New(Config{Roots: []string{t.TempDir()}, Accounts: accounts, Attribute: bySession(nil),
		Now: func() time.Time { return at(now) }, Loc: loc, Readings: readingsOf(m)})
}

func TestHistoryPeakPercentIsTheHighestReadingOfTheLocalDay(t *testing.T) {
	// mst is 7 hours behind utc: 03:00Z on the 26th is 20:00 on the 25th
	s := pctService(t, mst, "2026-10-27T21:00:00Z", map[string][]Reading{"work": {
		reading("2026-10-25T17:00:00Z", f(30), f(10)),
		reading("2026-10-25T20:00:00Z", f(80), f(15)),
		reading("2026-10-26T03:00:00Z", f(55), f(40)), // still the 25th locally
		reading("2026-10-26T07:30:00Z", f(10), f(41)), // 00:30 on the 26th
		reading("2026-10-26T09:00:00Z", nil, f(20)),
	}})
	h, err := s.History(3)
	if err != nil {
		t.Fatal(err)
	}
	var w HistoryAccount
	for _, a := range h.Accounts {
		if a.ID == "work" {
			w = a
		}
	}
	d25, d26, d27 := w.Days[0], w.Days[1], w.Days[2]
	if d25.Peak5hPct == nil || *d25.Peak5hPct != 80 || *d25.Peak7dPct != 40 {
		t.Errorf("25th: %+v", d25)
	}
	if d26.Peak5hPct == nil || *d26.Peak5hPct != 10 || *d26.Peak7dPct != 41 {
		t.Errorf("26th: %+v", d26)
	}
	if d27.Peak5hPct != nil || d27.Peak7dPct != nil {
		t.Errorf("27th has no reading: %+v", d27)
	}
}

func TestHistoryPeakPercentAcrossTheFallBackDay(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	// 1 Nov 2026 is 25 hours long in Los Angeles: PDT ends at 09:00Z
	s := pctService(t, loc, "2026-11-02T20:00:00Z", map[string][]Reading{"work": {
		reading("2026-10-31T06:59:00Z", f(5), nil),  // 23:59 on the 30th
		reading("2026-10-31T07:00:00Z", f(20), nil), // 00:00 on the 31st
		reading("2026-11-01T07:00:00Z", f(30), nil), // 00:00 PDT on the 1st
		reading("2026-11-01T08:30:00Z", f(40), nil), // 01:30 PDT
		reading("2026-11-01T09:30:00Z", f(70), nil), // 01:30 PST, the repeated hour
		reading("2026-11-02T07:59:00Z", f(50), nil), // 23:59 PST, the last minute of the 1st
		reading("2026-11-02T08:00:00Z", f(15), nil), // 00:00 PST on the 2nd
	}})
	h, err := s.History(4)
	if err != nil {
		t.Fatal(err)
	}
	var days []HistoryDay
	for _, a := range h.Accounts {
		if a.ID == "work" {
			days = a.Days
		}
	}
	want := []struct {
		day string
		pct *float64
	}{{"2026-10-30", f(5)}, {"2026-10-31", f(20)}, {"2026-11-01", f(70)}, {"2026-11-02", f(15)}}
	if len(days) != 4 {
		t.Fatalf("%+v", days)
	}
	for i, w := range want {
		g := days[i]
		if g.Day != w.day || g.Peak5hPct == nil || *g.Peak5hPct != *w.pct {
			t.Errorf("day %d: %+v want %s %v", i, g, w.day, *w.pct)
		}
	}
}

func TestHistoryTodayFallsBackToTheLiveReadingOnlyWithoutReadings(t *testing.T) {
	s := pctService(t, mst, "2026-10-27T21:00:00Z", map[string][]Reading{"personal": {
		reading("2026-10-27T16:00:00Z", f(60), f(9)),
	}})
	s.cfg.Limits = func(id string) (*float64, *float64) { return f(41.5), f(12) }
	h, _ := s.History(2)
	for _, a := range h.Accounts {
		today := a.Days[1]
		switch a.ID {
		case "personal":
			if *today.Peak5hPct != 60 || *today.Peak7dPct != 9 {
				t.Errorf("personal: %+v", today)
			}
		case "work":
			if *today.Peak5hPct != 41.5 || *today.Peak7dPct != 12 {
				t.Errorf("work: %+v", today)
			}
		}
		if a.Days[0].Peak5hPct != nil {
			t.Errorf("%s yesterday: %+v", a.ID, a.Days[0])
		}
	}
}

func TestDayHasPeakAndLastFivePercent(t *testing.T) {
	root := t.TempDir()
	write(t, root, "-p", "s1", at("2026-10-27T20:00:00Z"), asst("2026-10-27T16:00:00Z", "s1", "/p", "a", 10, 0))
	s := New(Config{Roots: []string{root}, Accounts: accounts, Attribute: bySession(map[string]string{"s1": "work"}),
		Now: func() time.Time { return at("2026-10-27T21:00:00Z") }, Loc: mst,
		Readings: readingsOf(map[string][]Reading{"work": {
			reading("2026-10-26T20:00:00Z", f(99), nil), // the day before
			reading("2026-10-27T10:00:00Z", f(20), nil),
			reading("2026-10-27T14:00:00Z", f(70), nil),
			reading("2026-10-27T18:00:00Z", f(35), nil),
		}})})
	sum, err := s.Day("2026-10-27")
	if err != nil {
		t.Fatal(err)
	}
	a := sum.Accounts[0]
	if a.ID != "work" || a.Peak5hPct == nil || *a.Peak5hPct != 70 || a.Last5hPct == nil || *a.Last5hPct != 35 {
		t.Errorf("%+v %+v %+v", a.ID, a.Peak5hPct, a.Last5hPct)
	}
	// a day with no readings says null, not 0
	write(t, root, "-p", "s2", at("2026-10-25T20:00:00Z"), asst("2026-10-25T16:00:00Z", "s1", "/p", "b", 4, 0))
	old, _ := s.Day("2026-10-25")
	if len(old.Accounts) != 1 || old.Accounts[0].Peak5hPct != nil || old.Accounts[0].Last5hPct != nil {
		t.Errorf("%+v", old.Accounts)
	}
}

func TestDayTodayFallsBackToTheLiveReading(t *testing.T) {
	root := t.TempDir()
	write(t, root, "-p", "s1", at("2026-10-27T20:00:00Z"), asst("2026-10-27T16:00:00Z", "s1", "/p", "a", 10, 0))
	s := New(Config{Roots: []string{root}, Accounts: accounts, Attribute: bySession(map[string]string{"s1": "work"}),
		Now: func() time.Time { return at("2026-10-27T21:00:00Z") }, Loc: mst,
		Limits: func(string) (*float64, *float64) { return f(44), nil }})
	sum, _ := s.Day("")
	a := sum.Accounts[0]
	if a.Peak5hPct == nil || *a.Peak5hPct != 44 || *a.Last5hPct != 44 {
		t.Errorf("%+v %+v", a.Peak5hPct, a.Last5hPct)
	}
}
