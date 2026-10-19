package dashboard

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestHelpBoxListsEveryKey(t *testing.T) {
	box := stripAll(asciiTheme().helpBox(100, 30, 0))
	out := strings.Join(box, "\n")
	rows := map[string]string{
		"up/down, k/j": "move in the focused pane", "tab": "switch pane", "[ ] or 1-5": "switch view",
		"h/l, left/right": "switch history tab", "enter": "run claude as the account", "s": "sign in",
		"e": "edit the budget", "r": "rename the account (accounts pane)", "d": "sign out the account (accounts pane)",
		"x": "stop the selected session", "/": "filter accounts",
		"a": "toggle ascii mode", "?": "show or hide this help", "esc": "close help or filter",
		"backspace": "go back from a project", "q": "quit", "ctrl+c": "quit from anywhere",
	}
	for key, desc := range rows {
		l := lineWith(box, desc)
		if l == "" || !strings.Contains(l, " "+key+" ") {
			t.Errorf("help row for %q should sit on the line with %q:\n%s", key, desc, out)
		}
	}
	if !strings.Contains(box[0], "Keys") || !strings.Contains(box[len(box)-1], "esc closes") {
		t.Errorf("the box is titled Keys and ends with the close hint:\n%s", out)
	}
}

func TestHelpExplainsThatEnterOpensAProject(t *testing.T) {
	box := stripAll(asciiTheme().helpBox(100, 30, 0))
	first := lineWith(box, " enter ")
	rest := lineWith(box, "Projects table is focused")
	if !strings.Contains(first, "run claude as the account, or open the project") || rest == "" {
		t.Errorf("enter runs claude, or opens a project when the History Projects table is focused:\n%s", strings.Join(box, "\n"))
	}
}

// the box at 100x30: a key column, a description column, long descriptions
// wrapped under themselves, a blank line above and below
func TestHelpBoxLayoutAt100x30(t *testing.T) {
	box := stripAll(asciiTheme().helpBox(100, 30, 0))
	row := func(key, desc string) string {
		return "|  " + key + strings.Repeat(" ", 17-len(key)) + desc + strings.Repeat(" ", 46-len(desc)) + "  |"
	}
	want := []string{
		"+- Keys " + strings.Repeat("-", 60) + "+",
		"|" + strings.Repeat(" ", 67) + "|",
		row("up/down, k/j", "move in the focused pane"),
		row("tab", "switch pane"),
		row("[ ] or 1-5", "switch view"),
		row("h/l, left/right", "switch history tab"),
		row("enter", "run claude as the account, or open the project"),
		row("", "when the History Projects table is focused"),
		row("s", "sign in"),
		row("e", "edit the budget"),
		row("r", "rename the account (accounts pane)"),
		row("d", "sign out the account (accounts pane)"),
		row("x", "stop the selected session"),
		row("/", "filter accounts"),
		row("a", "toggle ascii mode"),
		row("?", "show or hide this help"),
		row("esc", "close help or filter"),
		row("esc, backspace", "go back from a project"),
		row("q", "quit"),
		row("ctrl+c", "quit from anywhere"),
		"|" + strings.Repeat(" ", 67) + "|",
		"+- esc closes " + strings.Repeat("-", 54) + "+",
	}
	if len(box) != len(want) {
		t.Fatalf("%d lines, want %d:\n%s", len(box), len(want), strings.Join(box, "\n"))
	}
	for i := range want {
		if box[i] != want[i] {
			t.Errorf("line %d =\n%q, want\n%q", i, box[i], want[i])
		}
	}
}

func TestHelpBoxFitsEveryScreen(t *testing.T) {
	for w := 20; w <= 120; w++ {
		for _, h := range []int{16, 20, 24, 30} {
			for _, at := range []int{0, 5, 1000} {
				box := asciiTheme().helpBox(w, h, at)
				if len(box) > h {
					t.Fatalf("%dx%d at %d: box has %d lines", w, h, at, len(box))
				}
				for i, l := range box {
					if got := ansi.StringWidth(ansi.Strip(l)); got > w-2 {
						t.Fatalf("%dx%d at %d: line %d is %d wide: %q", w, h, at, i, got, l)
					}
				}
			}
		}
	}
}

func TestHelpBoxDoesNotScrollWhenItFits(t *testing.T) {
	for _, sz := range [][2]int{{100, 30}, {80, 24}} {
		th := asciiTheme()
		if th.helpLayout(sz[0], sz[1]).maxAt() != 0 {
			t.Errorf("%dx%d: every row fits, there is nothing to scroll", sz[0], sz[1])
		}
		box := stripAll(th.helpBox(sz[0], sz[1], 7))
		if !strings.Contains(box[len(box)-1], "esc closes") || strings.Contains(box[len(box)-1], "scrolls") {
			t.Errorf("%dx%d: no scroll hint when there is nothing to scroll: %q", sz[0], sz[1], box[len(box)-1])
		}
		for i, l := range box[1 : len(box)-1] {
			if strings.HasSuffix(l, "#") {
				t.Errorf("%dx%d: row %d shows a scroll thumb: %q", sz[0], sz[1], i, l)
			}
		}
	}
}

func TestHelpBoxScrollsOnAShortScreen(t *testing.T) {
	th := asciiTheme()
	top := stripAll(th.helpBox(60, 16, 0))
	if len(top) != 16 || !strings.Contains(top[1], "up/down, k/j") || strings.Contains(strings.Join(top, "\n"), "quit from anywhere") {
		t.Fatalf("at the top the first rows show and the last ones do not:\n%s", strings.Join(top, "\n"))
	}
	if !strings.Contains(top[15], "up/down scrolls, esc closes") {
		t.Errorf("the bottom line says how to scroll: %q", top[15])
	}
	if !strings.HasSuffix(top[1], "#") || strings.HasSuffix(top[14], "#") {
		t.Errorf("the thumb starts at the top:\n%s", strings.Join(top, "\n"))
	}
	end := stripAll(th.helpBox(60, 16, 1000))
	if !strings.Contains(end[14], "quit from anywhere") || !strings.HasSuffix(end[14], "#") || strings.HasSuffix(end[1], "#") {
		t.Errorf("an offset past the end shows the end, with the thumb at the bottom:\n%s", strings.Join(end, "\n"))
	}
	seen := map[string]bool{}
	for at := 0; at <= th.helpLayout(60, 16).maxAt(); at++ {
		for _, l := range stripAll(th.helpBox(60, 16, at)) {
			for _, r := range helpRows {
				if strings.Contains(l, " "+r[0]+" ") {
					seen[r[0]] = true
				}
			}
		}
	}
	for _, r := range helpRows {
		if !seen[r[0]] {
			t.Errorf("scrolling never shows the row for %q", r[0])
		}
	}
}

func TestOverlayCentersTheBoxOverTheFrame(t *testing.T) {
	th := asciiTheme()
	var base []string
	for i := 0; i < 30; i++ {
		base = append(base, strings.Repeat(".", 100))
	}
	box := th.helpBox(100, 30, 0)
	bw := ansi.StringWidth(box[0])
	out := stripAll(overlay(base, box, 100, 30))
	if len(out) != 30 {
		t.Fatalf("overlay must keep the frame height, got %d lines", len(out))
	}
	found := false
	for _, l := range out {
		if i := strings.Index(l, "+- Keys"); i >= 0 {
			found = true
			// a blank gutter column sits on each side of the box
			if want := (100-(bw+2))/2 + 1; i != want {
				t.Errorf("box starts at column %d, want %d", i, want)
			}
			if l[i-1] != ' ' || l[i+bw] != ' ' {
				t.Errorf("the gutter columns are blank: %q", l)
			}
		}
	}
	if !found {
		t.Fatal("the box never showed up")
	}
	if out[0] != strings.Repeat(".", 100) {
		t.Error("rows above the box stay untouched")
	}
}
