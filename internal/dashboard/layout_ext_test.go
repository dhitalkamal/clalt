package dashboard_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dhitalkamal/clalt/internal/dashboard"
)

var testSizes = [][2]int{
	{60, 16}, {61, 17}, {71, 20}, {72, 20}, {80, 24}, {95, 30}, {96, 30}, {100, 30}, {140, 40}, {200, 60},
}

func TestEveryStateFitsEverySize(t *testing.T) {
	for _, ascii := range []bool{true, false} {
		for _, sz := range testSizes {
			w, h := sz[0], sz[1]
			walk(config{ascii: ascii}, w, h, func(label string, m tea.Model) {
				checkFrame(t, fmt.Sprintf("%s ascii=%v", label, ascii), screen(m), w, h)
			})
			base := sized(config{ascii: ascii}, w, h)
			help, _ := press(base, "?")
			flt, _ := press(base, "/")
			flt = typeText(flt, "sh")
			none := typeText(flt, "zz")
			flashed, cmd := press(base, "enter")
			flashed = settle(flashed, cmd)
			budget, _ := press(base, "e")
			stop, _ := press(base, "2", "x")
			for name, m := range map[string]tea.Model{"help": help, "filter": flt, "no match": none, "flash": flashed, "budget": budget, "stop": stop} {
				checkFrame(t, fmt.Sprintf("%s %dx%d ascii=%v", name, w, h, ascii), screen(m), w, h)
			}
		}
	}
}

// renders every state at many sizes around the breakpoints and checks that no
// frame overflows or loses a line
func TestSizeSweep(t *testing.T) {
	widths := []int{59, 60, 61, 65, 71, 72, 73, 80, 94, 95, 96, 97, 120}
	heights := []int{15, 16, 17, 19, 24, 30, 40}
	for _, w := range widths {
		for _, h := range heights {
			walk(config{}, w, h, func(label string, m tea.Model) {
				checkFrame(t, label, screen(m), w, h)
			})
			base := sized(config{}, w, h)
			help, _ := press(base, "?")
			filt, _ := press(base, "/")
			filt = typeText(filt, "a")
			for name, m := range map[string]tea.Model{"help": help, "filter": filt} {
				checkFrame(t, fmt.Sprintf("%s %dx%d", name, w, h), screen(m), w, h)
			}
		}
	}
}

func TestPaneWidthsFollowTheBreakpoints(t *testing.T) {
	wide := "+- Accounts " + strings.Repeat("-", 13) + "+ +- robin -"
	narrow := "+- Accounts " + strings.Repeat("-", 9) + "+ +- robin -"
	cases := []struct {
		w    int
		want string
	}{{140, wide}, {100, wide}, {96, wide}, {95, narrow}, {80, narrow}, {72, narrow}}
	for _, c := range cases {
		if out := screen(sized(config{ascii: true}, c.w, 30)); !strings.Contains(out, c.want) {
			t.Errorf("width %d: expected %q in\n%s", c.w, c.want, out)
		}
	}
}

func TestSingleTabbedPaneUnder72Columns(t *testing.T) {
	for _, w := range []int{71, 64, 60} {
		m := sized(config{ascii: true}, w, 20)
		list := screen(m)
		if !strings.Contains(list, "morgan") || strings.Contains(list, "Overview") {
			t.Errorf("width %d: should open on the accounts pane alone:\n%s", w, list)
		}
		m, _ = press(m, "tab")
		detail := screen(m)
		if !strings.Contains(detail, "Overview") || strings.Contains(detail, "morgan") {
			t.Errorf("width %d: tab should switch to the detail pane alone:\n%s", w, detail)
		}
		m, _ = press(m, "tab")
		if !strings.Contains(screen(m), "morgan") {
			t.Errorf("width %d: tab again should come back to accounts", w)
		}
	}
	m := sized(config{ascii: true}, 64, 20)
	m, _ = press(m, "shift+tab")
	if out := screen(m); !strings.Contains(out, "Overview") || strings.Contains(out, "morgan") {
		t.Errorf("shift+tab should toggle the pane like tab:\n%s", out)
	}
}

func TestAccountRowsMatchTheApprovedList(t *testing.T) {
	out := screen(sized(config{ascii: true}, 100, 30))
	rows := []string{
		"| > robin     26%   * 2  | |",
		// morgan's login is due in 2 days, so the list flags it
		"|   morgan    22% !      | |",
		"|   casey     95% ! * 1  | |",
		"|   jordan     5%        | |",
		"|   harriet  signed out  | |",
		"|   jordan has the most  | |",
		"|   room left            | |",
	}
	for _, r := range rows {
		if !strings.Contains(out, r) {
			t.Errorf("missing left pane row %q in\n%s", r, out)
		}
	}
	narrow := screen(sized(config{ascii: true}, 80, 24))
	if strings.Contains(narrow, "* 2") || strings.Contains(narrow, "* 1") {
		t.Error("the narrow layout should drop the session dots")
	}
	if !strings.Contains(narrow, "!") || !strings.Contains(narrow, "no login") {
		t.Error("the near limit marker and the short signed out word stay in the narrow layout")
	}
}

func TestBarCellsFollowThePercent(t *testing.T) {
	out := screen(sized(config{ascii: true}, 100, 30))
	barRe := regexp.MustCompile(`[#=-]{10,}`)
	b5 := barRe.FindString(lineWith(t, out, "  26%  resets"))
	b7 := barRe.FindString(lineWith(t, out, "  42%  resets"))
	if len(b5) != len(b7) || len(b5) < 20 || strings.Count(b5, "#") >= strings.Count(b7, "#") {
		t.Errorf("both bars share one width and 42%% fills more than 26%%: %q vs %q", b5, b7)
	}
}

func TestScrollWhenTheRightPaneIsTallerThanTheScreen(t *testing.T) {
	m := sized(config{ascii: true}, 60, 16)
	m, _ = press(m, "tab")
	top := screen(m)
	if strings.Contains(top, "Running") || !strings.Contains(top, "Signed in.") {
		t.Fatalf("expected the top of the overview first:\n%s", top)
	}
	thumbShown := func(out string) bool {
		for _, l := range strings.Split(out, "\n") {
			if strings.HasSuffix(strings.TrimRight(l, " "), "#") {
				return true
			}
		}
		return false
	}
	if !thumbShown(top) {
		t.Error("overflowing content should show a scroll thumb on the border")
	}
	for i := 0; i < 12; i++ {
		m, _ = press(m, "j")
	}
	bottom := screen(m)
	if !strings.Contains(bottom, "Running") || strings.Contains(bottom, "Signed in.") || !strings.Contains(bottom, "Overview") {
		t.Errorf("j should scroll the overview to its end and keep the tab row:\n%s", bottom)
	}
	up, _ := press(m, "k")
	if screen(up) == bottom {
		t.Error("one k after hitting the end must scroll back up, so the offset may not grow past the end")
	}
	for i := 0; i < 12; i++ {
		m, _ = press(m, "k")
	}
	if !strings.Contains(screen(m), "Signed in.") {
		t.Error("k should scroll back to the top")
	}
	if thumbShown(screen(sized(config{ascii: true}, 100, 30))) {
		t.Error("no scroll thumb when everything fits")
	}
}

func TestSessionsCursorScrollsIntoView(t *testing.T) {
	m := sized(config{ascii: true}, 60, 16)
	m, _ = press(m, "2", "tab", "j")
	if out := screen(m); !strings.Contains(out, "> waiting") {
		t.Errorf("cursor row must stay visible after moving:\n%s", out)
	}
}

func TestNothingDrawsBeforeTheFirstSizeArrives(t *testing.T) {
	m := dashboard.New(t.Context(), nilSource{}, nil, dashboard.Options{ASCII: true})
	if got := screen(m); strings.Contains(got, "Accounts") || got != "" {
		t.Errorf("no size yet, nothing to draw, got %q", got)
	}
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if !strings.Contains(screen(m), "Accounts") {
		t.Error("the size message should trigger a full render")
	}
}

func TestTooSmallShowsAMessageUnder60By16(t *testing.T) {
	const msg = "Terminal too small. Make it at least 60 by 16."
	for _, sz := range [][2]int{{50, 14}, {59, 40}, {200, 15}, {30, 10}} {
		out := screen(sized(config{ascii: true}, sz[0], sz[1]))
		if flat := strings.Join(strings.Fields(out), " "); !strings.Contains(flat, msg) {
			t.Errorf("%dx%d: expected the too small message, got:\n%s", sz[0], sz[1], out)
		}
		if strings.Contains(out, "Accounts") {
			t.Errorf("%dx%d: dashboard should not render when too small", sz[0], sz[1])
		}
	}
	out := screen(sized(config{ascii: true}, 60, 16))
	if strings.Contains(out, "too small") || !strings.Contains(out, "Accounts") {
		t.Error("60x16 is the minimum and should render the dashboard")
	}
}
