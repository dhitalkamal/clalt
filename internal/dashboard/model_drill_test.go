package dashboard

import (
	"fmt"
	"strings"
	"testing"
)

// toProjects reaches the History projects table with the right pane focused
var toProjects = []string{"4", "tab", "l"}

func projectsModel(act *fakeActions) model {
	m, _ := actionModel(act)
	return pressSettle(m, toProjects...)
}

func drilledModel(act *fakeActions) model { return pressSettle(projectsModel(act), "enter") }

func footerOf(m model) string {
	lines := strings.Split(Screen(m), "\n")
	return lines[len(lines)-1]
}

// longProjects is one account with more projects than the pane has rows, each
// with more activity than it has rows
func longProjects(projects, rows int) Snapshot {
	a := testAccount()
	a.ID = "a1"
	a.History.Projects = nil
	for i := 0; i < projects; i++ {
		p := Share{Name: fmt.Sprintf("proj-%02d", i), Tokens: 1_000_000, Cost: 10, Sessions: rows}
		for d := 0; d < rows; d++ {
			p.Activity = append(p.Activity, Activity{Day: fmt.Sprintf("Day %02d", d), Model: "Opus 5.5", Sessions: 1, Tokens: 1000, Cost: 0.5})
		}
		a.History.Projects = append(a.History.Projects, p)
	}
	return Snapshot{Accounts: []Account{a}}
}

func TestEnterOnAProjectRowOpensItsDrillIn(t *testing.T) {
	act := &fakeActions{}
	m := pressSettle(projectsModel(act), "j", "enter")
	if out := Screen(m); m.drill.project != "web" || !strings.Contains(out, "Projects / web") {
		t.Errorf("enter on the second row opens web:\n%s", out)
	}
	if got := act.log(); len(got) != 0 {
		t.Errorf("opening a project is not a run: %v", got)
	}
}

func TestEnterOnTheAccountsPaneStillRuns(t *testing.T) {
	act := &fakeActions{}
	m := pressSettle(projectsModel(act), "tab", "enter")
	if got := act.log(); len(got) != 1 || got[0] != "run a1" || m.drill.open() {
		t.Errorf("enter with the accounts pane focused runs, it never opens a project: %v", got)
	}
}

func TestEnterElsewhereInTheRightPaneStillRuns(t *testing.T) {
	for name, keys := range map[string][]string{
		"overview":       {"tab"},
		"sessions":       {"2", "tab"},
		"limits":         {"3", "tab"},
		"history days":   {"4", "tab"},
		"history models": {"4", "tab", "l", "l"},
		"profile":        {"5", "tab"},
		// the projects tab is remembered when the view changes
		"sessions after projects": {"4", "tab", "l", "2"},
	} {
		act := &fakeActions{}
		m, _ := actionModel(act)
		m = pressSettle(m, append(keys, "enter")...)
		if got := act.log(); len(got) != 1 || got[0] != "run a1" || m.drill.open() {
			t.Errorf("%s: enter should run a1: %v", name, got)
		}
	}
}

func TestEnterOnAnEmptyProjectsTableStillRuns(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	m.snap.Accounts[0].History.Projects = nil
	m = pressSettle(m, append(toProjects, "enter")...)
	if got := act.log(); len(got) != 1 || got[0] != "run a1" || m.drill.open() {
		t.Errorf("no project under the cursor, so enter runs: %v", got)
	}
}

func TestEnterOnASignedOutAccountNeverDrills(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	m = pressSettle(m, "j", "j", "j", "j", "4", "tab", "l", "enter")
	if m.drill.open() || len(act.log()) != 0 || !strings.Contains(m.flash.text, "signed out") {
		t.Errorf("signed out: drill %q, log %v, flash %q", m.drill.project, act.log(), m.flash.text)
	}
}

func TestEscAndBackspaceGoBackToTheSameRow(t *testing.T) {
	for _, back := range []string{"esc", "backspace"} {
		m := pressSettle(projectsModel(&fakeActions{}), "j", "j", "enter")
		if m.drill.project != "other" {
			t.Fatalf("setup: the drill-in is on %q", m.drill.project)
		}
		m = pressSettle(m, back)
		out := Screen(m)
		if m.drill.open() || m.rowCur != 2 || !strings.Contains(out, "[Projects]") || !strings.Contains(out, "> other") {
			t.Errorf("%s goes back to the table with other selected:\n%s", back, out)
		}
	}
}

func TestBackKeepsTheRowAndTheDrillStartsAtTheTop(t *testing.T) {
	src := &fakeSource{snap: longProjects(40, 40)}
	m := pressSettle(loadedModel(src, &fakeActions{}, testOptions(), 100, 20), toProjects...)
	for i := 0; i < 25; i++ {
		m = pressSettle(m, "j")
	}
	before := m.scroll
	m = pressSettle(m, "enter")
	if out := Screen(m); !strings.Contains(out, "Projects / proj-25") || !strings.Contains(out, "Day 00") || m.scroll != 0 {
		t.Fatalf("the drill-in starts at its first row:\n%s", out)
	}
	m = pressSettle(m, "j", "j", "j", "j", "j", "j", "j", "j")
	if out := Screen(m); strings.Contains(out, "Day 00") {
		t.Errorf("down scrolls the drill-in:\n%s", out)
	}
	m = pressSettle(m, "esc")
	if out := Screen(m); m.rowCur != 25 || m.scroll != before || !strings.Contains(out, "> proj-25") {
		t.Errorf("back lands on row 25 as it was (scroll %d, was %d):\n%s", m.scroll, before, out)
	}
}

func TestEscNeverQuitsInTheDrillAndCtrlCDoes(t *testing.T) {
	m := drilledModel(&fakeActions{})
	for _, k := range []string{"esc", "backspace"} {
		if _, cmd := Press(m, k); Quits(cmd) {
			t.Errorf("%s must never quit", k)
		}
	}
	back := pressSettle(m, "esc")
	if _, cmd := Press(back, "esc"); Quits(cmd) {
		t.Error("esc on the table must not quit either")
	}
	if _, cmd := Press(m, "ctrl+c"); !Quits(cmd) {
		t.Error("ctrl+c quits from the drill-in")
	}
	if _, cmd := Press(m, "q"); !Quits(cmd) {
		t.Error("q quits from the drill-in like anywhere else")
	}
}

func TestEnterInsideTheDrillDoesNothing(t *testing.T) {
	act := &fakeActions{}
	next, cmd := Press(drilledModel(act), "enter")
	if cmd != nil || len(act.log()) != 0 || !next.(model).drill.open() {
		t.Errorf("nothing to open or run inside the drill-in: cmd %v, log %v", cmd != nil, act.log())
	}
}

func TestEnterOnTheAccountsPaneRunsEvenWithTheDrillOpen(t *testing.T) {
	act := &fakeActions{}
	pressSettle(drilledModel(act), "tab", "enter")
	if got := act.log(); len(got) != 1 || got[0] != "run a1" {
		t.Errorf("the accounts pane still runs: %v", got)
	}
}

func TestTheDrillClosesWhenTheContextChanges(t *testing.T) {
	for name, keys := range map[string][]string{
		"another account": {"tab", "j"},
		"next view":       {"]"},
		"previous view":   {"["},
		"a view by key":   {"1"},
	} {
		if m := pressSettle(drilledModel(&fakeActions{}), keys...); m.drill.open() {
			t.Errorf("%s: the drill-in should close", name)
		}
	}
	m := pressSettle(drilledModel(&fakeActions{}), "tab", "j", "tab")
	if out := Screen(m); !strings.Contains(out, "[Projects]") || !strings.Contains(detailTitle(out), "morgan") {
		t.Errorf("the next account shows its own table:\n%s", out)
	}
}

func detailTitle(out string) string { return strings.Split(out, "\n")[1] }

func TestTheDrillStaysThroughKeysThatDoNotChangeTheContext(t *testing.T) {
	for name, keys := range map[string][]string{
		"left and right are not tabs here": {"h", "l", "left", "right"},
		"scrolling":                        {"j", "j", "k", "down", "up"},
		"pane there and back":              {"tab", "tab"},
		"enter":                            {"enter"},
		"ascii toggle":                     {"a"},
		"help opened and closed":           {"?", "esc"},
	} {
		m := pressSettle(drilledModel(&fakeActions{}), keys...)
		if m.drill.project != "clalt" {
			t.Errorf("%s: the drill-in is on %q, want clalt", name, m.drill.project)
		}
	}
}

func TestEscOnTheAccountsPaneLeavesTheDrillAlone(t *testing.T) {
	m := pressSettle(drilledModel(&fakeActions{}), "tab", "esc", "backspace")
	if m.drill.project != "clalt" {
		t.Error("esc and backspace act on the pane that has focus")
	}
}

func TestEscBacksOutOneLevelAtATime(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	m = pressSettle(m, "/", "r", "o", "b", "enter", "4", "tab", "l", "enter")
	if !m.drill.open() || m.filter != "rob" {
		t.Fatalf("setup: drill %q, filter %q", m.drill.project, m.filter)
	}
	if m = pressSettle(m, "esc"); m.drill.open() || m.filter != "rob" {
		t.Errorf("the first esc leaves the drill-in and keeps the filter: %q %q", m.drill.project, m.filter)
	}
	if m = pressSettle(m, "esc"); m.filter != "" {
		t.Errorf("the second esc clears the filter: %q", m.filter)
	}
}

func TestReloadKeepsTheDrillWhileTheProjectIsThere(t *testing.T) {
	m := drilledModel(&fakeActions{})
	m, _ = upd(m, loadedMsg{snap: fiveSnapshot()})
	if m.drill.project != "clalt" {
		t.Errorf("a refresh must not close the drill-in: %q", m.drill.project)
	}
}

func TestReloadWithoutTheProjectFallsBackToTheTable(t *testing.T) {
	m := pressSettle(projectsModel(&fakeActions{}), "j", "enter")
	snap := fiveSnapshot()
	for i, a := range snap.Accounts {
		if ps := a.History.Projects; len(ps) == 3 {
			snap.Accounts[i].History.Projects = append(ps[:1:1], ps[2:]...)
		}
	}
	m, _ = upd(m, loadedMsg{snap: snap})
	out := Screen(m)
	if m.drill.open() || m.rowCur != 1 || !strings.Contains(out, "[Projects]") || !strings.Contains(out, "> other") {
		t.Errorf("the project is gone, so the table shows on the same row:\n%s", out)
	}
}

func TestTheDrillClosesWhenTheAccountSignsOut(t *testing.T) {
	m := drilledModel(&fakeActions{})
	snap := fiveSnapshot()
	snap.Accounts[0].SignedIn = false
	m, _ = upd(m, loadedMsg{snap: snap})
	if m.drill.open() {
		t.Error("a signed out account has no projects to drill into")
	}
}

func TestTheDrillFooterOffersBackInsteadOfRun(t *testing.T) {
	m := drilledModel(&fakeActions{})
	foot := footerOf(m)
	for _, want := range []string{"esc back", "tab pane", "[ ] or 1-5 view", "? keys", "q quit"} {
		if !strings.Contains(foot, want) {
			t.Errorf("drill footer %q is missing %q", foot, want)
		}
	}
	for _, gone := range []string{"enter", "sign in", "budget", "find"} {
		if strings.Contains(foot, gone) {
			t.Errorf("drill footer %q should not offer %q", foot, gone)
		}
	}
	if foot := footerOf(pressSettle(m, "tab")); !strings.Contains(foot, "enter run") {
		t.Errorf("with the accounts pane focused the normal hints are back: %q", foot)
	}
}

func TestDrillHintsKeepBackToTheEnd(t *testing.T) {
	for w := 10; w <= 100; w++ {
		line := plainText(hintsLine(drillHints, w))
		if len(line) > w || !strings.Contains(line, "esc back") {
			t.Fatalf("w=%d: %q", w, line)
		}
	}
}
