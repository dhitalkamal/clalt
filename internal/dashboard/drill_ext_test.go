package dashboard_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var (
	drillRowRe   = regexp.MustCompile(`(\w{3} \d+)\s+(Opus 5\.5|Sonnet 5\.5)\s+(\d+)\s+(\S+)\s+~\$(\d+)\.(\d\d)`)
	drillTotalRe = regexp.MustCompile(`Total\s+~\$(\d+)\.(\d\d) \((\d+)% of cost\)\s+(\S+) tokens\s+(\d+) sessions`)
)

func toDrill(w, h int, extra ...string) string {
	m := sized(config{ascii: true, noColor: true}, w, h)
	m, _ = press(m, append([]string{"4", "tab", "l"}, extra...)...)
	return screen(m)
}

func TestDrillRowsAddUpToTheProjectTotalsOnDemoData(t *testing.T) {
	out := toDrill(100, 80, "enter")
	if !strings.Contains(out, "Projects / clalt") {
		t.Fatalf("enter opens the first project:\n%s", out)
	}
	total := drillTotalRe.FindStringSubmatch(out)
	if total == nil {
		t.Fatalf("no totals line:\n%s", out)
	}
	wantCents := atoi(t, total[1])*100 + atoi(t, total[2])
	wantSessions := atoi(t, total[5])
	cents, sessions, rows := 0, 0, 0
	for _, l := range strings.Split(out, "\n") {
		if r := drillRowRe.FindStringSubmatch(l); r != nil {
			cents += atoi(t, r[5])*100 + atoi(t, r[6])
			sessions += atoi(t, r[3])
			rows++
		}
	}
	if rows < 10 || cents != wantCents || sessions != wantSessions {
		t.Errorf("%d rows add up to %d cents and %d sessions, the totals say %d and %d:\n%s",
			rows, cents, sessions, wantCents, wantSessions, out)
	}
	if !strings.Contains(out, "Last 30 days, newest first.") {
		t.Error("a tall terminal shows the whole list and the note under it")
	}
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("not a number: %q", s)
	}
	return n
}

func TestDrillListsNewestFirst(t *testing.T) {
	out := toDrill(100, 80, "enter")
	first := drillRowRe.FindStringSubmatch(out)
	if first == nil || first[1] != "Wed 15" {
		t.Errorf("the first row is today, Wed 15: %v", first)
	}
}

func TestDrillOpensTheProjectUnderTheCursorAndBackKeepsIt(t *testing.T) {
	out := toDrill(100, 30, "j", "enter")
	crumb := regexp.MustCompile(`Projects / (\S+)`).FindStringSubmatch(out)
	if crumb == nil || crumb[1] == "clalt" {
		t.Fatalf("one row down is the second project:\n%s", out)
	}
	back := toDrill(100, 30, "j", "enter", "esc")
	if !strings.Contains(back, "[Projects]") || lineWith(t, back, "> "+crumb[1]) == "" {
		t.Errorf("esc shows the projects table with %s selected:\n%s", crumb[1], back)
	}
}

func TestDrillEnterOnTheAccountsPaneStillRunsTheDemoAction(t *testing.T) {
	m := sized(config{ascii: true, noColor: true}, 100, 30)
	m, _ = press(m, "4", "tab", "l", "tab")
	m, cmd := press(m, "enter")
	out := screen(settle(m, cmd))
	if strings.Contains(out, "Projects / ") || !strings.Contains(out, "Demo: would start claude as robin.") {
		t.Errorf("enter with the accounts pane focused runs and never opens a project:\n%s", out)
	}
}
