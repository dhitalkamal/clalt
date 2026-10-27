package wrapper

import (
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/pick"
)

var clock = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

func lim(five, seven float64) pick.Limits {
	w := func(u float64, d time.Duration) pick.Window {
		return pick.Window{Used: u, ResetsAt: clock.Add(d), Known: true}
	}
	return pick.Limits{FiveHour: w(five, 2*time.Hour), SevenDay: w(seven, 72*time.Hour), ObservedAt: clock.Add(-time.Minute)}
}

func cand(id string, auto bool) pick.Candidate {
	return pick.Candidate{ID: id, Label: id, SignedIn: true, AutoSwitch: auto}
}

// input is a session on work at 93% with personal at 20% beside it, both with
// auto-switch on: the case that moves. each test spoils one thing.
func input() PolicyInput {
	by := map[string]pick.Limits{"work": lim(93, 40), "personal": lim(20, 30), "spare": lim(50, 10)}
	return PolicyInput{
		Now: clock, Cfg: Config{}.withDefaults(), Cwd: "/x",
		Current:    cand("work", true),
		Candidates: []pick.Candidate{cand("work", true), cand("personal", true), cand("spare", true)},
		Limits:     func(id string) (pick.Limits, bool) { l, ok := by[id]; return l, ok },
	}
}

func TestDecideMovesToTheAccountWithMostRoom(t *testing.T) {
	d, ok := Decide(input())

	if !ok || d.To.ID != "personal" {
		t.Fatalf("Decide = %+v, %v, want a move to personal", d, ok)
	}
	if !strings.Contains(d.Why, "5-hour window at 93%") || !strings.Contains(d.Why, "90%") {
		t.Errorf("why = %q", d.Why)
	}
}

func TestDecideStaysBelowTheThreshold(t *testing.T) {
	in := input()
	in.Limits = func(id string) (pick.Limits, bool) {
		if id == "work" {
			return lim(89, 10), true
		}
		return lim(0, 0), true
	}
	if d, ok := Decide(in); ok {
		t.Errorf("moved at 89%%: %+v", d)
	}
}

func TestDecideMovesWhenClaudeReportedTheLimit(t *testing.T) {
	in := input()
	in.LimitHit = true
	in.Limits = func(id string) (pick.Limits, bool) {
		if id == "work" {
			return lim(10, 10), true // what the feed says is not what claude said
		}
		return lim(60, 10), true
	}

	d, ok := Decide(in)

	if !ok || !strings.Contains(d.Why, "claude reported the limit") {
		t.Errorf("Decide = %+v, %v", d, ok)
	}
}

func TestDecideNeedsAutoSwitchOnBothSides(t *testing.T) {
	in := input()
	in.Current.AutoSwitch = false
	if _, ok := Decide(in); ok {
		t.Error("moved off an account that has auto-switch off")
	}
	in = input()
	in.Candidates = []pick.Candidate{cand("work", true), cand("personal", false), cand("spare", false)}
	if _, ok := Decide(in); ok {
		t.Error("moved to accounts that have auto-switch off")
	}
}

func TestDecideNeedsClearlyMoreRoom(t *testing.T) {
	in := input()
	by := map[string]pick.Limits{"work": lim(93, 40), "personal": lim(75, 10)} // 25 against 7
	in.Candidates = []pick.Candidate{cand("work", true), cand("personal", true)}
	in.Limits = func(id string) (pick.Limits, bool) { l, ok := by[id]; return l, ok }
	// 25 left against 7 is 18 more, and the margin is 20
	if d, ok := Decide(in); ok {
		t.Errorf("moved for 18 points: %+v", d)
	}
	by["personal"] = lim(70, 10)
	if _, ok := Decide(in); !ok {
		t.Error("did not move for 23 points")
	}
}

func TestDecideLooksAtTheRoomOfTheSevenDayWindowToo(t *testing.T) {
	in := input()
	by := map[string]pick.Limits{"work": lim(93, 40), "personal": lim(5, 95)}
	in.Candidates = []pick.Candidate{cand("work", true), cand("personal", true)}
	in.Limits = func(id string) (pick.Limits, bool) { l, ok := by[id]; return l, ok }
	if d, ok := Decide(in); ok {
		t.Errorf("moved to an account whose week is nearly gone: %+v", d)
	}
}

func TestDecideNeedsLimitDataForTheTarget(t *testing.T) {
	in := input()
	in.Limits = func(id string) (pick.Limits, bool) {
		if id == "work" {
			return lim(95, 10), true
		}
		return pick.Limits{}, false
	}
	if d, ok := Decide(in); ok {
		t.Errorf("moved to an account nothing is known about: %+v", d)
	}
}

func TestDecideKeepsToTheFolderRules(t *testing.T) {
	in := input()
	p := cand("personal", true)
	p.Folders = []string{"/elsewhere"}
	in.Candidates = []pick.Candidate{cand("work", true), p, cand("spare", true)}

	d, ok := Decide(in)

	if !ok || d.To.ID != "spare" {
		t.Errorf("Decide = %+v, %v, want the one that may serve /x", d, ok)
	}
}

func TestDecideWaitsOutTheCooldownAfterAMove(t *testing.T) {
	in := input()
	in.LastMove = clock.Add(-10 * time.Minute)
	if _, ok := Decide(in); ok {
		t.Error("moved again ten minutes after a move")
	}
	in.LastMove = clock.Add(-31 * time.Minute)
	if _, ok := Decide(in); !ok {
		t.Error("still waiting after the cooldown")
	}
}

func TestDecideSkipsATargetThatFailedUntilTheCooldownIsOver(t *testing.T) {
	in := input()
	in.Blocked = map[string]time.Time{"personal": clock.Add(-5 * time.Minute)}
	d, ok := Decide(in)
	if !ok || d.To.ID != "spare" {
		t.Fatalf("Decide = %+v, %v, want spare", d, ok)
	}
	in.Blocked["personal"] = clock.Add(-time.Hour)
	if d, _ := Decide(in); d.To.ID != "personal" {
		t.Errorf("a target that failed an hour ago is still skipped: %+v", d)
	}
}

func TestDecideDoesNothingWithNoDataForTheCurrentAccount(t *testing.T) {
	in := input()
	in.Limits = func(id string) (pick.Limits, bool) {
		if id == "work" {
			return pick.Limits{}, false
		}
		return lim(0, 0), true
	}
	if _, ok := Decide(in); ok {
		t.Error("moved with no sign that the account is low")
	}
}

func TestDefaultsFillWhatIsZeroAndKeepWhatIsSet(t *testing.T) {
	c := Config{}.withDefaults()
	if c.Poll != 500*time.Millisecond || c.Settle != 1500*time.Millisecond || c.Grace != 15*time.Second ||
		c.Cooldown != 30*time.Minute || c.Threshold != 90 || c.Margin != 20 {
		t.Errorf("defaults = %+v", c)
	}
	if c := (Config{Threshold: 80, Poll: time.Second}).withDefaults(); c.Threshold != 80 || c.Poll != time.Second {
		t.Errorf("set values were replaced: %+v", c)
	}
}
