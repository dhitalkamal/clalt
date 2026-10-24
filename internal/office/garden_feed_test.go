package office

import (
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func snapWith(accts []AccountOut, sessions ...SessionOut) Snapshot {
	return Snapshot{V: Version, Accounts: accts, Sessions: sessions}
}

func subs(types ...string) []Subagent {
	var out []Subagent
	for _, t := range types {
		out = append(out, Subagent{Type: t, State: "running"})
	}
	return out
}

func deskTotals(t *testing.T, g *GardenFeed) (cabin map[string]int, ws, rs int) {
	t.Helper()
	l, _, err := g.Layout("rows")
	if err != nil {
		t.Fatal(err)
	}
	cabin = map[string]int{}
	for _, p := range l.Pavilions {
		d := p.Cols * p.Rows
		switch p.Type {
		case "cabin":
			cabin[p.Account] += d
		case "workspace":
			ws += d * 2
		case "research":
			rs += d * 2
		}
	}
	return
}

func TestGardenFeedCountsNeeds(t *testing.T) {
	clk := &fakeClock{time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)}
	g := NewGardenFeed(clk.now)
	var mains []SessionOut
	for i := 0; i < 5; i++ {
		mains = append(mains, SessionOut{Handle: "w" + string(rune('a'+i)), Account: "work"})
	}
	mains[0].Subagents = subs("Explore", "explore", "Plan", "code-reviewer", "general-purpose", "general-purpose", "")
	mains[0].Subagents = append(mains[0].Subagents, Subagent{Type: "general-purpose", State: "done"})
	g.Observe(snapWith([]AccountOut{{ID: "work", Label: "WORK"}, {ID: "home", Label: "HOME"}}, mains...))
	cabin, ws, rs := deskTotals(t, g)
	if cabin["work"] != 6 || cabin["home"] != 2 {
		t.Errorf("cabin desks %v, want work 6 (5 sessions round up to rows) and home 2", cabin)
	}
	if ws < 8 || rs < 4 {
		t.Errorf("pools below the minimum: ws %d rs %d", ws, rs)
	}
}

func TestGardenFeedPoolsFollowSubagents(t *testing.T) {
	clk := &fakeClock{time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)}
	g := NewGardenFeed(clk.now)
	acc := []AccountOut{{ID: "work", Label: "WORK"}}
	g.Observe(snapWith(acc))
	s := SessionOut{Handle: "h", Account: "work"}
	for i := 0; i < 12; i++ {
		s.Subagents = append(s.Subagents, Subagent{Type: "Explore", State: "running"})
	}
	for i := 0; i < 20; i++ {
		s.Subagents = append(s.Subagents, Subagent{Type: "general-purpose", State: "running"})
	}
	g.Observe(snapWith(acc, s))
	if _, ws, rs := deskTotals(t, g); ws != 8 || rs != 4 {
		t.Errorf("grew before the hold: ws %d rs %d", ws, rs)
	}
	clk.t = clk.t.Add(31 * time.Second)
	g.Observe(snapWith(acc, s))
	if _, ws, rs := deskTotals(t, g); ws < 20 || rs < 12 {
		t.Errorf("did not grow after the hold: ws %d rs %d", ws, rs)
	}
}

func TestGardenFeedAccountOrderAndUnknownSessions(t *testing.T) {
	g := NewGardenFeed(time.Now)
	g.Observe(snapWith([]AccountOut{{ID: "b", Label: "B"}, {ID: "a", Label: "A"}}, SessionOut{Handle: "x", Account: "ghost"}))
	l, rev, err := g.Layout("village")
	if err != nil || rev == "" {
		t.Fatalf("layout: %v %q", err, rev)
	}
	var order []string
	for _, p := range l.Pavilions {
		if p.Type == "cabin" {
			order = append(order, p.Account)
		}
	}
	if len(order) != 2 || order[0] != "b" || order[1] != "a" {
		t.Errorf("cabins %v, want snapshot order b a (a session of an unlisted account has no cabin)", order)
	}
}

func TestGardenFeedBeforeFirstSnapshot(t *testing.T) {
	if _, _, err := NewGardenFeed(time.Now).Layout("street"); err != nil {
		t.Fatal(err)
	}
}
