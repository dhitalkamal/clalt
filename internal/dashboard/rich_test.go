package dashboard

import (
	"strings"
	"testing"
)

func plainText(r rich) string {
	var sb strings.Builder
	for _, sp := range r {
		sb.WriteString(sp.s)
	}
	return sb.String()
}

func TestWrapNeverOverflowsAndKeepsTones(t *testing.T) {
	line := rich{dim("Signed in. "), warn("Re-login due in 2 days. "), dim("Last used 3h ago and some extra words")}
	for w := 1; w <= 60; w++ {
		for i, l := range wrap(line, w) {
			if l.width() > w {
				t.Fatalf("w=%d line %d is %d wide: %v", w, i, l.width(), l)
			}
			if len(l) > 0 && strings.HasPrefix(l[0].s, " ") && i > 0 {
				t.Fatalf("w=%d continuation line %d starts with a space", w, i)
			}
		}
	}
	sawWarn := false
	for _, l := range wrap(line, 20) {
		for _, sp := range l {
			if sp.t == toneWarn && strings.Contains(sp.s, "Re-login") {
				sawWarn = true
			}
		}
	}
	if !sawWarn {
		t.Error("wrapping should keep the warning tone on its words")
	}
	if got := wrap(rich{txt("abcdefghij")}, 4); len(got) != 3 || got[0].width() != 4 {
		t.Errorf("a long word should be cut to the width, got %v", got)
	}
	if got := wrap(nil, 10); len(got) != 0 {
		t.Errorf("wrapping nothing should give nothing, got %v", got)
	}
}

func TestHangIndentsContinuationLines(t *testing.T) {
	lines := hang(rich{dim("5h")}, 4, rich{txt("runs out around 01:10, about 35 minutes before it resets")}, 30)
	if len(lines) < 2 {
		t.Fatal("expected the body to wrap")
	}
	for i, l := range lines {
		if l.width() > 30 {
			t.Errorf("line %d is %d wide", i, l.width())
		}
		if i > 0 && l[0].s != "    " {
			t.Errorf("continuation line %d should start with the label indent, got %q", i, l[0].s)
		}
	}
	if got := hang(rich{dim("Folder")}, 11, nil, 40); len(got) != 1 || got[0].width() != 11 {
		t.Errorf("an empty body still yields the label, got %v", got)
	}
}

func TestFlowPacksWholeItemsAndNeverSplitsOne(t *testing.T) {
	items := []rich{{txt("~$47.10 (66% of cost)")}, {txt("12.3M tokens")}, {txt("40 sessions")}}
	plain := func(w int) []string {
		var out []string
		for _, l := range flow(items, 3, w) {
			out = append(out, plainText(l))
		}
		return out
	}
	if got := plain(80); len(got) != 1 || got[0] != "~$47.10 (66% of cost)   12.3M tokens   40 sessions" {
		t.Errorf("everything fits on one line: %q", got)
	}
	got := plain(35)
	if len(got) != 2 || got[0] != "~$47.10 (66% of cost)" || got[1] != "12.3M tokens   40 sessions" {
		t.Errorf("a number stays with its unit when the line breaks: %q", got)
	}
	if got := plain(14); len(got) != 4 || got[1] != "of cost)" || got[2] != "12.3M tokens" {
		t.Errorf("an item wider than the line wraps on its own, the rest stay whole: %q", got)
	}
	for w := 1; w <= 60; w++ {
		for i, l := range flow(items, 3, w) {
			if l.width() > w {
				t.Fatalf("w=%d line %d is %d wide: %q", w, i, l.width(), plainText(l))
			}
		}
	}
	if got := flow(nil, 3, 20); len(got) != 0 {
		t.Errorf("no items, no lines: %v", got)
	}
}

func TestFirstFit(t *testing.T) {
	long, short := rich{txt("0123456789")}, rich{txt("abc")}
	if got := firstFit(10, long, short); got.width() != 10 {
		t.Error("the first candidate that fits wins")
	}
	if got := firstFit(5, long, short); got.width() != 3 {
		t.Error("a later candidate is used when the first is too wide")
	}
	if got := firstFit(2, long, rich{txt("abcdef")}); got.width() != 2 {
		t.Error("with no fit the last candidate is clipped")
	}
}

func TestClipAndFit(t *testing.T) {
	r := rich{dim("abc"), warn("defg"), txt("hi")}
	if got := plainText(r.clip(5)); got != "abcde" {
		t.Errorf("clip(5) = %q", got)
	}
	if got := r.clip(5); got[len(got)-1].t != toneWarn {
		t.Error("a clipped span keeps its tone")
	}
	if got := r.clip(0); len(got) != 0 {
		t.Errorf("clip(0) should be empty, got %v", got)
	}
	if got := plainText(r.clip(50)); got != "abcdefghi" {
		t.Errorf("a wide clip changes nothing, got %q", got)
	}
	if got := r.fit(12); got.width() != 12 || !strings.HasSuffix(plainText(got), "hi   ") {
		t.Errorf("fit(12) should pad to 12, got %q", plainText(got))
	}
	if got := r.fit(4); got.width() != 4 {
		t.Errorf("fit(4) should clip to 4, got %d", got.width())
	}
	if gap(-3).s != "" {
		t.Error("a negative gap is empty")
	}
}

func TestDrawMergesNeighboursOfOneTone(t *testing.T) {
	p := newPalette(false)
	got := p.draw(rich{good("ab"), good("cd"), dim("ef"), txt("gh")})
	if n := strings.Count(got, "\x1b[32m"); n != 1 {
		t.Errorf("neighbours in one tone share one escape, found %d in %q", n, got)
	}
	if !strings.HasSuffix(got, "gh") {
		t.Errorf("default text carries no escape, got %q", got)
	}
	if p.draw(nil) != "" || p.draw(rich{txt("")}) != "" {
		t.Error("drawing nothing gives an empty string")
	}
}

func TestEllipsizeKeepsWithinWidth(t *testing.T) {
	r := rich{txt("~/Projects/personal/clalt"), dim(" (feat/dashboard-v2)")}
	if got := r.ellipsize(80, "..."); got.width() != r.width() {
		t.Error("text that fits is left alone")
	}
	for w := 5; w < r.width(); w++ {
		got := r.ellipsize(w, "...")
		if got.width() > w {
			t.Fatalf("w=%d: ellipsized text is %d wide", w, got.width())
		}
		if !strings.HasSuffix(plainText(got), "...") {
			t.Fatalf("w=%d: cut text should end with the ellipsis, got %q", w, plainText(got))
		}
	}
}

func TestAlignRightFillsTheWidth(t *testing.T) {
	got := alignRight(rich{bold("clalt")}, rich{dim("updated 4s ago")}, 40)
	if got.width() != 40 || !strings.HasSuffix(plainText(got), "updated 4s ago") {
		t.Errorf("right text should sit flush right, got %q", plainText(got))
	}
	if got := alignRight(rich{txt("left")}, rich{txt("right")}, 6); !strings.HasSuffix(plainText(got), "right") {
		t.Errorf("a tight fit still keeps the right text, got %q", plainText(got))
	}
}

func TestPromptAlternatesDimAndBold(t *testing.T) {
	got := prompt("Press ", "s", " to sign in.")
	if len(got) != 3 || got[0].t != toneDim || got[1].t != toneBold || got[2].t != toneDim {
		t.Errorf("prompt tones are dim, bold, dim: %v", got)
	}
	if plainText(got) != "Press s to sign in." {
		t.Errorf("prompt text = %q", plainText(got))
	}
}
