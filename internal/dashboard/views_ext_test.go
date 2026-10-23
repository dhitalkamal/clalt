package dashboard_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dhitalkamal/clalt/internal/dashboard"
)

// text only one view shows, used to tell the views apart
var signatures = []struct{ key, name, text string }{
	{"1", "overview", "Signed in. Re-login due in 23 days."},
	{"2", "sessions", "x stops the selected session."},
	{"3", "limits", "now 26%"},
	{"4", "history", "last 30 days"},
	{"5", "profile", "Folder"},
}

func TestFullLayoutListsViewsAndAccounts(t *testing.T) {
	for _, ascii := range []bool{false, true} {
		out := screen(sized(config{ascii: ascii}, 100, 30))
		var want []string
		want = append(want, viewNames...)
		want = append(want, accountNames...)
		want = append(want, "Accounts", "5 accounts, 3 running, updated 4s ago", "enter run", "q quit")
		for _, w := range want {
			if !strings.Contains(out, w) {
				t.Errorf("ascii=%v: view at 100x30 is missing %q", ascii, w)
			}
		}
	}
}

func TestEachViewShowsItsOwnText(t *testing.T) {
	m := sized(config{ascii: true}, 100, 30)
	for _, sig := range signatures {
		m, _ = press(m, sig.key)
		out := screen(m)
		if !strings.Contains(out, sig.text) {
			t.Errorf("%s view is missing %q", sig.name, sig.text)
		}
		for _, other := range signatures {
			if other.key != sig.key && strings.Contains(out, other.text) {
				t.Errorf("%s view also shows %s text %q", sig.name, other.name, other.text)
			}
		}
	}
}

func TestBracketKeysCycleViewsAndMarkTheActiveTab(t *testing.T) {
	m := sized(config{ascii: true}, 100, 30)
	if !strings.Contains(screen(m), "[Overview]") {
		t.Fatal("overview should start as the active tab, shown in brackets")
	}
	for i := 1; i < len(signatures); i++ {
		m, _ = press(m, "]")
		out := screen(m)
		if !strings.Contains(out, signatures[i].text) || !strings.Contains(out, "["+viewNames[i]+"]") {
			t.Errorf("] number %d: missing %q or the bracketed %s tab", i, signatures[i].text, viewNames[i])
		}
	}
	m, _ = press(m, "]")
	if !strings.Contains(screen(m), signatures[0].text) {
		t.Error("] after the last view should wrap to overview")
	}
	m, _ = press(m, "[")
	if !strings.Contains(screen(m), signatures[4].text) {
		t.Error("[ on overview should wrap to profile")
	}
}

func TestAccountSelectionMoves(t *testing.T) {
	m := sized(config{ascii: true}, 100, 30)
	steps := []struct{ key, want string }{
		{"down", "morgan"}, {"j", "casey"}, {"up", "morgan"}, {"k", "robin"}, {"k", "robin"},
		{"j", "morgan"}, {"j", "casey"}, {"j", "jordan"}, {"j", "harriet"}, {"j", "old-work"}, {"j", "old-work"},
	}
	for _, s := range steps {
		m, _ = press(m, s.key)
		out := screen(m)
		if !strings.Contains(rightTitle(out), "+- "+s.want+" ") {
			t.Fatalf("after %s the right pane should be titled %s:\n%s", s.key, s.want, out)
		}
		if !strings.Contains(lineWith(t, out, "> "+s.want), "> "+s.want) {
			t.Fatalf("marker should sit on %s", s.want)
		}
	}
}

func TestSessionRowsFollowTheRightPaneCursor(t *testing.T) {
	m := sized(config{ascii: true}, 100, 30)
	m, _ = press(m, "2")
	if !strings.Contains(screen(m), "> working") {
		t.Error("first session should carry the marker")
	}
	moved, _ := press(m, "tab", "j")
	if out := screen(moved); !strings.Contains(out, "> waiting") || strings.Contains(out, "> working") {
		t.Errorf("j on the right pane should move to the second session:\n%s", out)
	}
	back, _ := press(moved, "k")
	if !strings.Contains(screen(back), "> working") {
		t.Error("k should move back up")
	}
	acct, _ := press(m, "j")
	if !strings.Contains(rightTitle(screen(acct)), "morgan") {
		t.Error("j with the left pane focused should move the account")
	}
}

func TestHistoryTabsSwitchWithHLAndArrows(t *testing.T) {
	m := sized(config{ascii: true}, 100, 30)
	m, _ = press(m, "4")
	out := screen(m)
	for _, w := range []string{"[Days]", "Projects", "Models", "Wed 15", "~$2.14", "last 30 days  ~$71.20"} {
		if !strings.Contains(out, w) {
			t.Errorf("days tab is missing %q", w)
		}
	}
	if same, _ := press(m, "l"); !strings.Contains(screen(same), "[Days]") {
		t.Error("h and l only switch tabs while the right pane is focused")
	}
	m, _ = press(m, "tab", "l")
	out = screen(m)
	for _, w := range []string{"[Projects]", "web", "66%", "~$47.10", "13%", "share"} {
		if !strings.Contains(out, w) {
			t.Errorf("projects tab is missing %q", w)
		}
	}
	m, _ = press(m, "right")
	out = screen(m)
	for _, w := range []string{"[Models]", "Opus 5.5", "69%", "~$49.10", "Sonnet 5.5", "31%", "~$22.10", "cost share"} {
		if !strings.Contains(out, w) {
			t.Errorf("models tab is missing %q", w)
		}
	}
	m, _ = press(m, "l")
	if !strings.Contains(screen(m), "[Days]") {
		t.Error("l after models should wrap to days")
	}
	m, _ = press(m, "h", "left")
	if !strings.Contains(screen(m), "[Projects]") {
		t.Error("h then left from days should land on projects")
	}
}

func TestSignedOutAccountsShowTheSignInPromptInEveryView(t *testing.T) {
	for _, width := range []int{100, 80, 64} {
		m := sized(config{ascii: true}, width, 24)
		m, _ = press(m, "j", "j", "j", "j")
		if width < 72 {
			m, _ = press(m, "tab")
		}
		for i, key := range []string{"1", "2", "3", "4", "5"} {
			next, _ := press(m, key)
			out := screen(next)
			if !strings.Contains(out, "Signed out. Press s to sign in.") {
				t.Errorf("%dx24 %s view: no sign in prompt for harriet:\n%s", width, viewNames[i], out)
			}
			for _, noisy := range []string{"5h ", "Budget", "Today", "No sessions", "No usage history"} {
				if strings.Contains(out, noisy) {
					t.Errorf("%dx24 %s view: signed out accounts show only the prompt, found %q", width, viewNames[i], noisy)
				}
			}
		}
	}
}

// money pulls every ~$ amount out of a screen
func money(out string) []float64 {
	var vals []float64
	for _, m := range regexp.MustCompile(`~\$(\d+\.\d\d)`).FindAllStringSubmatch(out, -1) {
		v, _ := strconv.ParseFloat(m[1], 64)
		vals = append(vals, v)
	}
	return vals
}

func TestOverviewTotalsAgreeWithTheDaysTable(t *testing.T) {
	m := sized(config{ascii: true}, 100, 30)
	overview := screen(m)
	today := money(lineWith(t, overview, "Today  "))
	week := money(lineWith(t, overview, "Week  "))
	days := money(screen(mustPress(m, "4")))
	if len(today) != 1 || len(week) != 1 || len(days) < 11 {
		t.Fatalf("could not read the amounts: today %v week %v days %v", today, week, days)
	}
	if today[0] != days[0] {
		t.Errorf("the overview says %.2f today, the days table %.2f", today[0], days[0])
	}
	sum := 0.0
	for _, d := range days[:7] {
		sum += d
	}
	if diff := sum - week[0]; diff > 0.005 || diff < -0.005 {
		t.Errorf("the overview says %.2f for the week, the first seven days add up to %.2f", week[0], sum)
	}
	for _, want := range []string{"today  $2.14 of $10.00", "week  $18.40 of $50.00"} {
		if !strings.Contains(overview, want) {
			t.Errorf("the budget rows should spend the same amounts, missing %q", want)
		}
	}
}

func TestProjectsAndModelsAddUpToTheThirtyDayTotal(t *testing.T) {
	m := sized(config{ascii: true}, 100, 30)
	days := screen(mustPress(m, "4"))
	total := money(lineWith(t, days, "last 30 days"))
	projects := money(screen(mustPress(m, "4", "tab", "l")))
	models := money(screen(mustPress(m, "4", "tab", "l", "l")))
	sum := func(v []float64) float64 {
		s := 0.0
		for _, x := range v {
			s += x
		}
		return s
	}
	if len(total) != 1 || len(projects) != 6 || len(models) != 2 {
		t.Fatalf("could not read the amounts: %v %v %v", total, projects, models)
	}
	for name, got := range map[string]float64{"projects": sum(projects), "models": sum(models)} {
		if diff := got - total[0]; diff > 0.005 || diff < -0.005 {
			t.Errorf("%s add up to %.2f, the 30 day total is %.2f", name, got, total[0])
		}
	}
}

func mustPress(m tea.Model, keys ...string) tea.Model {
	next, _ := press(m, keys...)
	return next
}

func TestAsciiModeIsAscii(t *testing.T) {
	sizes := [][2]int{{100, 30}, {80, 24}, {64, 20}, {60, 16}, {50, 14}}
	for _, sz := range sizes {
		walk(config{ascii: true}, sz[0], sz[1], func(label string, m tea.Model) {
			if out := screen(m); !dashboard.IsASCII(out) {
				t.Errorf("%s: non-ascii output in ascii mode", label)
			}
		})
	}
	m := sized(config{ascii: true}, 100, 30)
	help, _ := press(m, "?")
	filter, _ := press(m, "/")
	filter = typeText(filter, "sh")
	flashed, cmd := press(m, "enter")
	flashed = settle(flashed, cmd)
	budget, _ := press(m, "e")
	stop, _ := press(m, "2", "x")
	for name, mm := range map[string]tea.Model{"help": help, "filter": filter, "flash": flashed, "budget": budget, "stop": stop} {
		if !dashboard.IsASCII(screen(mm)) {
			t.Errorf("%s: non-ascii output in ascii mode", name)
		}
	}
}

func TestAsciiKeyTogglesBothWays(t *testing.T) {
	m := sized(config{}, 100, 30)
	if dashboard.IsASCII(screen(m)) {
		t.Fatal("unicode mode should draw non-ascii glyphs")
	}
	m, _ = press(m, "a")
	if !dashboard.IsASCII(screen(m)) {
		t.Fatal("a should switch to ascii")
	}
	m, _ = press(m, "a")
	if dashboard.IsASCII(screen(m)) {
		t.Fatal("a again should switch back to unicode")
	}
}
