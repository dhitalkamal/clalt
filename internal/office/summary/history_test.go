package summary

import (
	"os"
	"testing"
	"time"
)

// work uses 100 at 10:00Z on the 25th, 50 at 12:00Z and 30 at 17:00Z (5h apart
// from the first two: the 5h window at 17:00 holds only the last), personal 7.
func historyFixture(t *testing.T) *Service {
	root := t.TempDir()
	mt := at("2026-10-27T20:00:00Z")
	write(t, root, "-p", "s1", mt,
		asst("2026-10-25T17:00:00Z", "s1", "/p", "a", 60, 40),
		asst("2026-10-25T19:00:00Z", "s1", "/p", "b", 30, 20),
		asst("2026-10-26T01:30:00Z", "s1", "/p", "c", 20, 10), // 18:30 local on the 25th
		asst("2026-10-27T16:00:00Z", "s1", "/p", "d", 5, 5),
	)
	write(t, root, "-p", "s2", mt, asst("2026-10-26T16:00:00Z", "s2", "/p", "e", 4, 3))
	return newService(root, "2026-10-27T21:00:00Z", map[string]string{"s1": "work", "s2": "personal"})
}

func TestHistoryDaysPerAccount(t *testing.T) {
	s := historyFixture(t)
	h, err := s.History(3)
	if err != nil {
		t.Fatal(err)
	}
	if h.Days != 3 || h.From != "2026-10-25" || h.To != "2026-10-27" || len(h.Accounts) != 2 {
		t.Fatalf("%+v", h)
	}
	p, w := h.Accounts[0], h.Accounts[1]
	if p.ID != "personal" || w.ID != "work" || len(w.Days) != 3 {
		t.Fatalf("%+v", h.Accounts)
	}
	d25, d26, d27 := w.Days[0], w.Days[1], w.Days[2]
	if d25.Day != "2026-10-25" || d25.Tokens != 100+50+30 || d25.Messages != 3 {
		t.Errorf("25th: %+v", d25)
	}
	// 10:00, 12:00 and 18:30 local: the first two are inside one 5h window (150),
	// the third is 6.5h after the second (30 alone)
	if d25.Peak5hTokens != 150 || d25.Peak7dTokens != 180 {
		t.Errorf("25th peaks: %+v", d25)
	}
	if d26.Tokens != 0 || d26.Peak5hTokens != 0 || d26.Messages != 0 {
		t.Errorf("26th: %+v", d26)
	}
	if d27.Tokens != 10 || d27.Peak5hTokens != 10 || d27.Peak7dTokens != 190 {
		t.Errorf("27th: %+v", d27)
	}
	if p.Days[1].Tokens != 7 || p.Days[0].Tokens != 0 {
		t.Errorf("personal: %+v", p.Days)
	}
	if d25.Peak5hPct != nil || d27.Peak7dPct != nil {
		t.Errorf("percents should be null without a reading")
	}
}

func TestHistoryPeakUsesRecordsBeforeTheRange(t *testing.T) {
	s := historyFixture(t)
	h, _ := s.History(1)
	// the 7d peak on the 27th counts the 25th too
	if got := h.Accounts[1].Days[0].Peak7dTokens; got != 190 {
		t.Errorf("7d peak: %d", got)
	}
}

func TestHistoryLiveReadingOnlyOnToday(t *testing.T) {
	s := historyFixture(t)
	five, seven := 41.5, 12.0
	s.cfg.Limits = func(id string) (*float64, *float64) {
		if id == "work" {
			return &five, &seven
		}
		return nil, nil
	}
	h, _ := s.History(2)
	w := h.Accounts[1]
	if w.Days[0].Peak5hPct != nil || w.Days[1].Peak5hPct == nil || *w.Days[1].Peak5hPct != 41.5 || *w.Days[1].Peak7dPct != 12 {
		t.Errorf("%+v", w.Days)
	}
	if h.Accounts[0].Days[1].Peak5hPct != nil {
		t.Errorf("personal has no reading")
	}
}

func TestHistoryListedAccountWithoutUsageIsFlat(t *testing.T) {
	root := t.TempDir()
	s := newService(root, "2026-10-27T21:00:00Z", nil)
	h, _ := s.History(2)
	if len(h.Accounts) != 2 || len(h.Accounts[0].Days) != 2 || h.Accounts[0].Days[0].Tokens != 0 {
		t.Errorf("%+v", h.Accounts)
	}
}

func TestHistoryBounds(t *testing.T) {
	s := historyFixture(t)
	for _, n := range []int{-1, 91, 1000} {
		if _, err := s.History(n); err != ErrBadDays {
			t.Errorf("%d: %v", n, err)
		}
	}
	if h, err := s.History(0); err != nil || h.Days != 30 {
		t.Errorf("default: %v %d", err, h.Days)
	}
	if h, err := s.History(90); err != nil || len(h.Accounts[0].Days) != 90 {
		t.Errorf("90: %v", err)
	}
}

func TestHistoryDaysAcrossDST(t *testing.T) {
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Skip("no tz database")
	}
	root := t.TempDir()
	write(t, root, "-p", "s1", at("2026-11-02T20:00:00Z"),
		asst("2026-11-01T12:00:00Z", "s1", "/p", "a", 1, 0), // 06:00 local, after the fall back on 1 Nov
		asst("2026-10-31T12:00:00Z", "s1", "/p", "b", 2, 0))
	s := New(Config{Roots: []string{root}, Accounts: accounts, Attribute: bySession(map[string]string{"s1": "work"}),
		Now: func() time.Time { return at("2026-11-02T20:00:00Z") }, Loc: loc})
	h, _ := s.History(3)
	w := h.Accounts[1].Days
	if w[0].Day != "2026-10-31" || w[0].Tokens != 2 || w[1].Day != "2026-11-01" || w[1].Tokens != 1 || w[2].Day != "2026-11-02" {
		t.Errorf("%+v", w)
	}
}

func TestServiceCachesForThirtySeconds(t *testing.T) {
	root := t.TempDir()
	write(t, root, "-p", "s1", at("2026-10-27T20:00:00Z"), asst("2026-10-27T16:00:00Z", "s1", "/p", "a", 10, 0))
	now := at("2026-10-27T21:00:00Z")
	s := New(Config{Roots: []string{root}, Accounts: accounts, Attribute: bySession(map[string]string{"s1": "work"}),
		Now: func() time.Time { return now }, Loc: mst})
	first, _ := s.Day("2026-10-27")
	// a new line lands, the cached reply still stands
	p := write(t, root, "-p", "s3", at("2026-10-27T20:30:00Z"), asst("2026-10-27T17:00:00Z", "s1", "/p", "z", 90, 0))
	now = now.Add(29 * time.Second)
	again, _ := s.Day("2026-10-27")
	if again.Totals.Tokens != first.Totals.Tokens {
		t.Fatalf("not cached: %d", again.Totals.Tokens)
	}
	now = now.Add(2 * time.Second)
	later, _ := s.Day("2026-10-27")
	if later.Totals.Tokens != 100 {
		t.Fatalf("not refreshed: %d", later.Totals.Tokens)
	}
	// another day has its own entry
	if other, _ := s.Day("2026-10-26"); other.Totals.Tokens != 0 || other.Day != "2026-10-26" {
		t.Errorf("other day: %+v", other.Totals)
	}
	_ = os.Remove(p)
}
