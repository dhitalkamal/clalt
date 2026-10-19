package dashboard_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dhitalkamal/clalt/internal/dashboard"
)

func TestFilterNarrowsTheListAndShowsTheTextAboveIt(t *testing.T) {
	m := sized(config{ascii: true}, 100, 30)
	m, _ = press(m, "/")
	if !strings.Contains(screen(m), "/ type to filter") {
		t.Error("an empty filter should show a placeholder")
	}
	m = typeText(m, "sh")
	out := screen(m)
	for _, gone := range []string{"| > robin", "|   morgan", "|   jordan"} {
		if strings.Contains(out, gone) {
			t.Errorf("filter sh should hide the row %q", gone)
		}
	}
	filterRow, listRow := -1, -1
	for i, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "/ sh") {
			filterRow = i
		}
		if strings.Contains(l, "|   harriet") {
			listRow = i
		}
	}
	if filterRow < 0 || listRow < 0 || filterRow >= listRow {
		t.Errorf("filter text should sit above the list (filter row %d, list row %d)", filterRow, listRow)
	}
	if !strings.Contains(rightTitle(out), "casey") {
		t.Error("selection should move to the first match")
	}
}

func TestFilterBackspaceEscAndEnter(t *testing.T) {
	m := sized(config{ascii: true}, 100, 30)
	m, _ = press(m, "/")
	m = typeText(m, "sh")
	m, _ = press(m, "backspace")
	if !strings.Contains(screen(m), "/ s") {
		t.Error("backspace should drop the last character")
	}
	m, _ = press(m, "backspace")
	out := screen(m)
	for _, n := range accountNames {
		if !strings.Contains(out, n) {
			t.Errorf("empty filter should list %s", n)
		}
	}

	m = typeText(m, "sh")
	kept, cmd := press(m, "enter")
	if quits(cmd) || !strings.Contains(screen(kept), "/ sh") || strings.Contains(screen(kept), "morgan") {
		t.Error("enter should keep the filter and leave typing mode")
	}
	if _, cmd := press(kept, "q"); !quits(cmd) {
		t.Error("q should quit once the filter is no longer being typed")
	}
	cleared, cmd := press(kept, "esc")
	if quits(cmd) || strings.Contains(screen(cleared), "/ sh") || !strings.Contains(screen(cleared), "morgan") {
		t.Error("esc should clear a kept filter")
	}
	cleared, _ = press(m, "esc")
	if strings.Contains(screen(cleared), "/ sh") || !strings.Contains(screen(cleared), "morgan") {
		t.Error("esc while typing should clear the filter")
	}
}

func TestFilterWithNoMatchIsHandled(t *testing.T) {
	m := sized(config{ascii: true}, 100, 30)
	m, _ = press(m, "/")
	m = typeText(m, "zzz")
	if out := screen(m); !strings.Contains(out, "no match") || !strings.Contains(out, "No account matches the filter.") {
		t.Errorf("expected a no match line and a hint, got:\n%s", out)
	}
	kept, _ := press(m, "enter")
	for _, key := range []string{"enter", "s", "e", "x", "r", "d"} {
		next, cmd := press(kept, key)
		out := screen(settle(next, cmd))
		if quits(cmd) || !strings.Contains(out, "No account selected.") || strings.Contains(out, "Demo:") {
			t.Errorf("%s with nothing to act on only says so:\n%s", key, out)
		}
	}
	for _, key := range []string{"tab", "j", "k", "]", "[", "2", "?", "a", "h", "l", "esc"} {
		next, cmd := press(kept, key)
		if quits(cmd) {
			t.Errorf("%s must not quit", key)
		}
		checkFrame(t, "no match then "+key, screen(settle(next, cmd)), 100, 30)
	}
}

func TestSlashFocusesTheAccountsPane(t *testing.T) {
	m := sized(config{ascii: true}, 64, 20)
	m, _ = press(m, "tab")
	if out := screen(m); !strings.Contains(out, "Overview") {
		t.Fatal("setup: expected the detail pane")
	}
	m, _ = press(m, "/")
	m = typeText(m, "an")
	if out := screen(m); !strings.Contains(out, "/ an") || !strings.Contains(out, "casey") {
		t.Errorf("/ should bring the accounts pane forward so the filter is visible:\n%s", out)
	}
}

func TestKeptFilterHintsAtEscape(t *testing.T) {
	m := sized(config{ascii: true}, 100, 30)
	m, _ = press(m, "/")
	m = typeText(m, "sh")
	m, _ = press(m, "enter")
	if !strings.Contains(screen(m), "esc clear filter") {
		t.Error("a kept filter should be announced in the footer")
	}
}

func TestFilterIgnoresALeadingSpace(t *testing.T) {
	m := sized(config{ascii: true}, 100, 30)
	m, _ = press(m, "/", "space", "a")
	if out := screen(m); !strings.Contains(out, "| / a ") || strings.Contains(out, "/  a") {
		t.Errorf("a leading space should not enter the filter:\n%s", out)
	}
	m, _ = press(m, "space")
	m = typeText(m, "s")
	if !strings.Contains(screen(m), "/ a s") {
		t.Error("spaces after the first character are kept")
	}
}

func TestEachDemoActionFlashesItsNotice(t *testing.T) {
	cases := []struct {
		keys []string
		want string
	}{
		{[]string{"enter"}, "Demo: would start claude as robin."},
		{[]string{"s"}, "Demo: would sign in to robin."},
		{[]string{"2", "x", "y"}, "Demo: would stop that session."},
		{[]string{"e", "enter", "enter"}, "Demo: would set robin's budget to $10.00 a day and $50.00 a week."},
		{[]string{"j", "j", "j", "j", "s"}, "Demo: would sign in to harriet."},
		{[]string{"j", "enter"}, "Demo: would start claude as morgan."},
	}
	for _, c := range cases {
		m := sized(config{ascii: true}, 100, 30)
		m, cmd := press(m, c.keys...)
		m = settle(m, cmd)
		if out := screen(m); !strings.Contains(out, c.want) {
			t.Errorf("%v: expected flash %q in\n%s", c.keys, c.want, out)
		}
	}
	m := sized(config{ascii: true}, 100, 30)
	m, cmd := press(m, "x")
	if out := screen(settle(m, cmd)); strings.Contains(out, "would stop") || !strings.Contains(out, "No session to stop here.") {
		t.Error("x outside the sessions view must not claim to stop anything")
	}
}

func TestFlashExpiresAndAnOldTimerDoesNotClearANewerFlash(t *testing.T) {
	m := demoModel(dashboard.Options{ASCII: true, NoColor: true, FlashFor: 5 * time.Millisecond}, 100, 30)
	flash := func(m tea.Model, key string) (tea.Model, tea.Cmd) {
		m, cmd := press(m, key)
		return m.Update(dashboard.RunCmd(cmd, time.Second))
	}
	m, first := flash(m, "enter")
	if !strings.Contains(screen(m), "Demo: would start claude as robin.") {
		t.Fatalf("setup: the first flash is showing:\n%s", screen(m))
	}
	m, second := flash(m, "s")
	stale, _ := m.Update(dashboard.RunCmd(first, time.Second))
	if !strings.Contains(screen(stale), "Demo: would sign in to robin.") {
		t.Error("an old timer must not clear a newer flash")
	}
	done, _ := stale.Update(dashboard.RunCmd(second, time.Second))
	if out := screen(done); strings.Contains(out, "Demo:") || !strings.Contains(out, "q quit") {
		t.Error("flash should clear and the key hints should return")
	}
}

func TestHelpOverlayListsEveryKeyIsCenteredAndCloses(t *testing.T) {
	m, _ := press(sized(config{ascii: true}, 100, 30), "?")
	out := screen(m)
	for _, w := range []string{
		"Keys", "up/down, k/j", "switch pane", "[ ] or 1-5", "switch view",
		"h/l, left/right", "switch history tab", "filter accounts", "toggle ascii mode",
		"close help or filter", "quit from anywhere",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("help overlay is missing %q", w)
		}
	}
	top := regexp.MustCompile(`\+- Keys -*\+`)
	for _, l := range strings.Split(out, "\n") {
		if loc := top.FindStringIndex(l); loc != nil {
			if d := loc[0] - (100 - loc[1]); d < -1 || d > 1 {
				t.Errorf("overlay is off center: %d columns left, %d right", loc[0], 100-loc[1])
			}
		}
	}
	moved, _ := press(m, "j", "tab", "2")
	closed, cmd := press(moved, "esc")
	if cout := screen(closed); quits(cmd) || strings.Contains(cout, "Keys") || !strings.Contains(cout, "Signed in.") {
		t.Error("esc should close the overlay and leave the dashboard untouched")
	}
	if toggled, _ := press(m, "?"); strings.Contains(screen(toggled), "up/down, k/j") {
		t.Error("? should toggle the overlay closed")
	}
}

func TestHelpOverlayFitsTheSmallestScreenAndScrolls(t *testing.T) {
	m, _ := press(sized(config{ascii: true}, 60, 16), "?")
	out := screen(m)
	if got := len(strings.Split(out, "\n")); got != 16 {
		t.Errorf("expected 16 lines, got %d", got)
	}
	for _, w := range []string{"up/down, k/j", "up/down scrolls, esc closes"} {
		if !strings.Contains(out, w) {
			t.Errorf("small help overlay is missing %q", w)
		}
	}
	for i := 0; i < 30; i++ {
		m, _ = press(m, "j")
	}
	end := screen(m)
	if got := len(strings.Split(end, "\n")); got != 16 || !strings.Contains(end, "quit from anywhere") {
		t.Errorf("scrolling down reaches the last row on the same 16 lines (%d):\n%s", got, end)
	}
}

func TestHelpListsEveryHandledKey(t *testing.T) {
	m, _ := press(sized(config{ascii: true}, 100, 30), "?")
	out := screen(m)
	rows := map[string]string{
		"up/down, k/j": "move in the focused pane", "tab": "switch pane", "[ ] or 1-5": "switch view",
		"h/l, left/right": "switch history tab", "enter": "run claude as the account", "s": "sign in",
		"e": "edit the budget", "r": "rename the account", "d": "sign out the account",
		"x": "stop the selected session", "/": "filter accounts",
		"a": "toggle ascii mode", "?": "show or hide this help", "esc": "close help or filter",
		"backspace": "go back from a project", "q": "quit", "ctrl+c": "quit from anywhere",
	}
	for key, desc := range rows {
		if l := lineWith(t, out, desc); !strings.Contains(l, " "+key+" ") {
			t.Errorf("help row for %q should sit on the line with %q: %q", key, desc, l)
		}
	}
	if !strings.Contains(out, "open the project") || !strings.Contains(out, "Projects table is focused") {
		t.Errorf("help says enter opens a project from the History Projects table:\n%s", out)
	}
}

func TestEscNeverQuitsAndQQuitsOnlyWhenNotTyping(t *testing.T) {
	base := sized(config{ascii: true}, 100, 30)
	typing, _ := press(base, "/")
	help, _ := press(base, "?")
	for name, m := range map[string]tea.Model{"normal": base, "filter": typing, "help": help} {
		if _, cmd := press(m, "esc"); quits(cmd) {
			t.Errorf("%s: esc must never quit", name)
		}
		if _, cmd := press(m, "ctrl+c"); !quits(cmd) {
			t.Errorf("%s: ctrl+c should quit", name)
		}
	}
	if _, cmd := press(base, "q"); !quits(cmd) {
		t.Error("q should quit from the normal state")
	}
	if _, cmd := press(typing, "q"); quits(cmd) {
		t.Error("q inside the filter is text, not quit")
	}
	if _, cmd := press(help, "q"); !quits(cmd) {
		t.Error("q should quit from the help overlay")
	}
}
