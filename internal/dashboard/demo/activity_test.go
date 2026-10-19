package demo

import (
	"testing"
)

// the drill-in lists a project's ledger rows, so they must add up to the
// project row they came from
func TestProjectActivityAddsUpToTheProjectRow(t *testing.T) {
	snap := loadSnapshot(t)
	for id := range ledgers {
		for _, p := range accountByID(t, snap, id).History.Projects {
			var cents, sessions int
			var tokens int64
			for _, act := range p.Activity {
				cents += toCents(act.Cost)
				tokens += act.Tokens
				sessions += act.Sessions
			}
			if cents != toCents(p.Cost) || tokens != p.Tokens || sessions != p.Sessions {
				t.Errorf("%s/%s: activity adds up to %d cents, %d tokens, %d sessions; the project row says %d, %d, %d",
					id, p.Name, cents, tokens, sessions, toCents(p.Cost), p.Tokens, p.Sessions)
			}
		}
	}
}

func TestProjectActivityIsNewestFirstAndUsesTheDayLabels(t *testing.T) {
	snap := loadSnapshot(t)
	for id := range ledgers {
		a := accountByID(t, snap, id)
		index := map[string]int{}
		for i, d := range a.History.Days {
			index[d.Label] = i
		}
		for _, p := range a.History.Projects {
			last := -1
			seen := map[string]bool{}
			for i, act := range p.Activity {
				if key := act.Day + "|" + act.Model; seen[key] {
					t.Errorf("%s/%s lists %s on %s twice, rows are one per day and model", id, p.Name, act.Model, act.Day)
				} else {
					seen[key] = true
				}
				day, ok := index[act.Day]
				if !ok {
					t.Fatalf("%s/%s row %d: %q is not one of the account's day labels", id, p.Name, i, act.Day)
				}
				if day < last {
					t.Errorf("%s/%s row %d (%s) is older than the row before it", id, p.Name, i, act.Day)
				}
				last = day
				if act.Model != opus && act.Model != sonnet {
					t.Errorf("%s/%s row %d has an unknown model %q", id, p.Name, i, act.Model)
				}
				if act.Sessions < 1 || act.Cost <= 0 {
					t.Errorf("%s/%s row %d is empty: %+v", id, p.Name, i, act)
				}
			}
		}
	}
}

func TestModelsHaveNoActivityAndEveryProjectHasSome(t *testing.T) {
	for id := range ledgers {
		h := accountByID(t, loadSnapshot(t), id).History
		for _, m := range h.Models {
			if len(m.Activity) != 0 {
				t.Errorf("%s: model %s lists activity, only projects drill in", id, m.Name)
			}
		}
		for _, p := range h.Projects {
			if len(p.Activity) == 0 {
				t.Errorf("%s: project %s has no activity to drill into", id, p.Name)
			}
		}
	}
}

func TestRobinsFirstProjectActivityStartsWithToday(t *testing.T) {
	a := accountByID(t, loadSnapshot(t), "robin")
	clalt := a.History.Projects[0]
	if clalt.Name != "clalt" {
		t.Fatalf("setup: the first project is %q", clalt.Name)
	}
	if first := clalt.Activity[0]; first.Day != "Wed 15" {
		t.Errorf("the newest clalt activity is %s, today is Wed 15", first.Day)
	}
	// the working session (1.10) and a finished one (0.42) are one row, they
	// share a day and a model
	first := clalt.Activity[0]
	if first.Model != opus || first.Sessions != 2 || toCents(first.Cost) != 152 {
		t.Errorf("today's clalt row = %+v, want 2 Opus sessions for 1.52", first)
	}
}
