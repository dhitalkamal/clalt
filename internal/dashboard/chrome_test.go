package dashboard

import (
	"strings"
	"testing"
)

func TestHintsLineMatchesTheApprovedFooters(t *testing.T) {
	cases := []struct {
		w    int
		want string
	}{
		{150, "  enter run   s sign in   e budget   r rename   d sign out   D delete   tab pane   [ ] or 1-5 view   / find   a ascii   ? keys   q quit"},
		{130, "  enter run  s sign in  e budget  r rename  d sign out  D delete  tab pane  [ ] or 1-5 view  / find  a ascii  ? keys  q quit"},
		{114, "  enter run  s sign in  e budget  r rename  d sign out  tab pane  [ ] or 1-5 view  / find  a ascii  ? keys  q quit"},
		{113, "  enter run  s sign in  e budget  r rename  d sign out  tab pane  [ ] or 1-5 view  / find  ? keys  q quit"},
		{100, "  enter run  s sign in  e budget  r rename  tab pane  [ ] or 1-5 view  / find  ? keys  q quit"},
		{80, "  enter run  s sign in  e budget  tab pane  [ ] or 1-5 view  ? keys  q quit"},
		{64, "  enter run  tab pane  [ ] or 1-5 view  ? keys  q quit"},
	}
	for _, c := range cases {
		if got := plainText(hintsLine(normalHints, c.w)); got != c.want {
			t.Errorf("width %d:\n got %q\nwant %q", c.w, got, c.want)
		}
	}
}

func TestHintsLineDropsTheLeastUsefulFirstAndNeverOverflows(t *testing.T) {
	order := []string{"D delete", "a ascii", "d sign out", "r rename", "/ find", "e budget", "s sign in", "enter run", "[ ] or 1-5 view", "tab pane", "q quit", "? keys"}
	prev := len(order)
	for w := 100; w >= 12; w-- {
		got := plainText(hintsLine(normalHints, w))
		if len([]rune(got)) > w {
			t.Fatalf("width %d: footer is %d wide: %q", w, len(got), got)
		}
		kept := 0
		for _, item := range order {
			if strings.Contains(got, item) {
				kept++
			}
		}
		if kept > prev {
			t.Fatalf("width %d: hints came back after being dropped: %q", w, got)
		}
		prev = kept
	}
	if got := plainText(hintsLine(normalHints, 30)); !strings.Contains(got, "? keys") {
		t.Errorf("help is the last hint to go, got %q", got)
	}
}

func TestKeptFilterHintOutlivesEveryHintButHelpAndQuit(t *testing.T) {
	lower := []string{"a ascii", "d sign out", "r rename", "/ find", "e budget", "s sign in", "enter run", "[ ] or 1-5 view", "tab pane"}
	sawClear := false
	for w := 140; w >= 20; w-- {
		got := plainText(hintsLine(keptHints(), w))
		if strings.Contains(got, "esc clear filter") {
			sawClear = true
			continue
		}
		if !sawClear {
			t.Fatalf("width %d: the way out of the filter should show wherever the full footer does: %q", w, got)
		}
		for _, h := range lower {
			if strings.Contains(got, h) {
				t.Errorf("width %d: %q is still shown after the filter hint went: %q", w, h, got)
			}
		}
	}
}

func TestHintsFirstItemIsBoldKeyThenDimText(t *testing.T) {
	line := hintsLine(helpHints, 80)
	var tones []tone
	for _, sp := range line {
		if strings.TrimSpace(sp.s) != "" {
			tones = append(tones, sp.t)
		}
	}
	if len(tones) < 2 || tones[0] != toneBold || tones[1] != toneDim {
		t.Errorf("key names are bold and descriptions dim, got tones %v", tones)
	}
}

func TestHeaderLinePicksTheLongestThatFits(t *testing.T) {
	long, mid, short := rich{dim("5 accounts, 3 running, updated 4s ago")}, rich{dim("5 accounts, 3 running")}, rich{dim("3 running")}
	cases := []struct {
		w    int
		want string
	}{
		{100, " clalt" + strings.Repeat(" ", 100-1-6-len("5 accounts, 3 running, updated 4s ago")) + "5 accounts, 3 running, updated 4s ago"},
		{40, " clalt" + strings.Repeat(" ", 40-1-6-len("5 accounts, 3 running")) + "5 accounts, 3 running"},
		{24, " clalt" + strings.Repeat(" ", 24-1-6-len("3 running")) + "3 running"},
		{12, " clalt"},
	}
	for _, c := range cases {
		if got := plainText(headerLine(c.w, long, mid, short)); strings.TrimRight(got, " ") != strings.TrimRight(c.want, " ") {
			t.Errorf("width %d: got %q, want %q", c.w, got, c.want)
		}
	}
}
