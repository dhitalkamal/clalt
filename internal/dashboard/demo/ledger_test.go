package demo

import (
	"context"
	"math"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/dashboard"
)

// atEpoch is the clock the demo tests run on
func atEpoch() time.Time { return Epoch }

func loadSnapshot(t *testing.T) dashboard.Snapshot {
	t.Helper()
	snap, err := Source(atEpoch).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return snap
}

func accountByID(t *testing.T, snap dashboard.Snapshot, id string) dashboard.Account {
	t.Helper()
	for _, a := range snap.Accounts {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("no account %q", id)
	return dashboard.Account{}
}

func toCents(dollars float64) int { return int(math.Round(dollars * 100)) }

type sums struct {
	cents, tokens, sessions int
}

func sumDays(days []dashboard.Day) sums {
	var s sums
	for _, d := range days {
		s.cents += toCents(d.Cost)
		s.tokens += int(d.Input + d.Output)
		s.sessions += d.Sessions
	}
	return s
}

func sumShares(items []dashboard.Share) sums {
	var s sums
	for _, it := range items {
		s.cents += toCents(it.Cost)
		s.tokens += int(it.Tokens)
		s.sessions += it.Sessions
	}
	return s
}

func sumRows(rows ledger) sums {
	var s sums
	for _, r := range rows {
		s.cents += r.cents
		s.tokens += int(r.input + r.output)
		s.sessions += r.sessions
	}
	return s
}

// every view of an account's usage comes from one ledger, so the totals must
// be the same whichever way they are cut
func TestEveryAccountsTotalsAgreeWhicheverWayTheyAreCut(t *testing.T) {
	snap := loadSnapshot(t)
	for id, rows := range ledgers {
		h := accountByID(t, snap, id).History
		want := sumRows(rows)
		days := sumDays(h.Days)
		if days != want {
			t.Errorf("%s: days %+v, ledger %+v", id, days, want)
		}
		projects, models := sumShares(h.Projects), sumShares(h.Models)
		if projects != want {
			t.Errorf("%s: projects %+v, ledger %+v", id, projects, want)
		}
		if models.cents != want.cents || models.tokens != want.tokens {
			t.Errorf("%s: models %+v, ledger %+v", id, models, want)
		}
		if len(h.Days) != 30 {
			t.Errorf("%s: %d days, want 30", id, len(h.Days))
		}
		var cache int64
		for _, d := range h.Days {
			cache += d.CacheRead + d.CacheWrite
		}
		var ledgerCache int64
		for _, r := range rows {
			ledgerCache += r.cacheRead + r.cacheWrite
		}
		if cache != ledgerCache {
			t.Errorf("%s: cache tokens %d in the days, %d in the ledger", id, cache, ledgerCache)
		}
	}
}

func TestTodayAndTheWeekAreSumsOfTheirRows(t *testing.T) {
	snap := loadSnapshot(t)
	for id, rows := range ledgers {
		a := accountByID(t, snap, id)
		var today, week ledger
		for _, r := range rows {
			if r.day == 0 {
				today = append(today, r)
			}
			if r.day < 7 {
				week = append(week, r)
			}
		}
		if got, want := sumDays(a.History.Days[:1]), sumRows(today); got != want {
			t.Errorf("%s: today's day row %+v, today's ledger rows %+v", id, got, want)
		}
		if got, want := sumDays(a.History.Days[:7]), sumRows(week); got != want {
			t.Errorf("%s: the first seven days %+v, the week's ledger rows %+v", id, got, want)
		}
	}
}

func TestEveryRowIsWellFormed(t *testing.T) {
	seen := map[string]string{}
	for id, rows := range ledgers {
		for i, r := range rows {
			if r.day < 0 || r.day > 29 || r.sessions < 1 || r.cents <= 0 {
				t.Errorf("%s row %d has a bad day, session count or cost: %+v", id, i, r)
			}
			if r.model != opus && r.model != sonnet {
				t.Errorf("%s row %d has an unknown model %q", id, i, r.model)
			}
			if r.input < 0 || r.output < 0 || r.cacheRead < 0 || r.cacheWrite < 0 {
				t.Errorf("%s row %d has negative tokens", id, i)
			}
			if r.session == "" {
				continue
			}
			if r.day != 0 || r.sessions != 1 {
				t.Errorf("%s row %d: a named session is one running session today", id, i)
			}
			if owner, dup := seen[r.session]; dup {
				t.Errorf("session %q appears in %s and %s", r.session, owner, id)
			}
			seen[r.session] = id
		}
	}
}

func TestRunningSessionsReconcileWithTheLedger(t *testing.T) {
	snap := loadSnapshot(t)
	total := 0
	for _, a := range snap.Accounts {
		var live int
		for _, s := range a.Sessions {
			total++
			rows := ledgers[a.ID].forSession(s.ID)
			if len(rows) != 1 {
				t.Errorf("%s: session %q has %d ledger rows, want 1", a.ID, s.ID, len(rows))
				continue
			}
			r := rows[0]
			if got := toCents(s.Cost); got != r.cents {
				t.Errorf("%s: session %q costs %d cents, its ledger row %d", a.ID, s.ID, got, r.cents)
			}
			if r.day != 0 {
				t.Errorf("%s: a running session belongs to today, row is day %d", a.ID, r.day)
			}
			if r.model != s.Model {
				t.Errorf("%s: session %q runs %s, the ledger says %s", a.ID, s.ID, s.Model, r.model)
			}
			if got := path.Base(s.Folder); got != r.project {
				t.Errorf("%s: session %q is in folder %q, the ledger project is %q", a.ID, s.ID, s.Folder, r.project)
			}
			live += r.cents
		}
		if len(a.History.Days) > 0 && live > toCents(a.History.Days[0].Cost) {
			t.Errorf("%s: running sessions cost %d cents, more than all of today (%d)", a.ID, live, toCents(a.History.Days[0].Cost))
		}
		if len(a.History.Days) > 0 && len(a.Sessions) > a.History.Days[0].Sessions {
			t.Errorf("%s: %d running sessions but only %d sessions today", a.ID, len(a.Sessions), a.History.Days[0].Sessions)
		}
	}
	if total != 3 {
		t.Errorf("the approved header says 3 running, found %d", total)
	}
}

func TestSharesAreSortedByCostAndShareNamesAreUnique(t *testing.T) {
	for _, a := range loadSnapshot(t).Accounts {
		for name, items := range map[string][]dashboard.Share{"projects": a.History.Projects, "models": a.History.Models} {
			names := map[string]bool{}
			for i, it := range items {
				if names[it.Name] {
					t.Errorf("%s %s: %q listed twice", a.ID, name, it.Name)
				}
				names[it.Name] = true
				if i > 0 && it.Cost > items[i-1].Cost {
					t.Errorf("%s %s: %q costs more than the row above it", a.ID, name, it.Name)
				}
			}
		}
	}
}

func TestDayLabelsRunBackFromTheAnchor(t *testing.T) {
	a := accountByID(t, loadSnapshot(t), "robin")
	got := []string{a.History.Days[0].Label, a.History.Days[1].Label, a.History.Days[9].Label, a.History.Days[29].Label}
	if want := []string{"Wed 15", "Tue 14", "Mon 6", "Tue 16"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("day labels = %v, want %v", got, want)
	}
	if got, want := strings.Join(a.Limits.Days, " "), "Thu Fri Sat Sun Mon Tue Wed"; got != want {
		t.Errorf("chart day labels = %q, want %q", got, want)
	}
}

func TestSignedOutAccountHasNoUsage(t *testing.T) {
	a := accountByID(t, loadSnapshot(t), "harriet")
	if a.SignedIn || len(a.History.Days) != 0 || len(a.Sessions) != 0 || len(a.Limits.Readings) != 0 {
		t.Errorf("a signed out account has nothing to report: %+v", a)
	}
	if _, ok := ledgers["harriet"]; ok {
		t.Error("harriet has no ledger rows")
	}
}

func TestChartEndsOnTheCurrentFiveHourPercent(t *testing.T) {
	for _, a := range loadSnapshot(t).Accounts {
		if !a.SignedIn {
			continue
		}
		r := a.Limits.Readings
		if len(r) != 56 || len(a.Limits.Days) != 7 {
			t.Errorf("%s: %d readings over %d days, want 56 over 7", a.ID, len(r), len(a.Limits.Days))
			continue
		}
		if got := int(r[len(r)-1]); got != a.Limits.Five.Percent {
			t.Errorf("%s: the last reading is %d, the 5h window says %d", a.ID, got, a.Limits.Five.Percent)
		}
	}
}
