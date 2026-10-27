package dashboard

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func asciiTheme() theme { return theme{p: newPalette(true), g: asciiGlyphs()} }

func stripAll(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = ansi.Strip(l)
	}
	return out
}

func TestBoxIsExactlyTheRequestedSize(t *testing.T) {
	th := asciiTheme()
	for _, c := range []struct{ w, h int }{{6, 3}, {20, 5}, {26, 16}, {80, 12}} {
		out := th.box(boxSpec{title: "Accounts", rows: []rich{{txt("hello")}}, w: c.w, h: c.h, padL: 1, padR: 1})
		if len(out) != c.h {
			t.Fatalf("%dx%d: %d lines", c.w, c.h, len(out))
		}
		for i, l := range out {
			if got := ansi.StringWidth(ansi.Strip(l)); got != c.w {
				t.Errorf("%dx%d: line %d is %d wide: %q", c.w, c.h, i, got, l)
			}
		}
	}
}

func TestBoxDrawsTitleAndBorder(t *testing.T) {
	out := stripAll(asciiTheme().box(boxSpec{title: "robin", rows: []rich{{txt("body")}}, w: 20, h: 4, padL: 1, padR: 1}))
	if want := "+- robin " + strings.Repeat("-", 10) + "+"; out[0] != want {
		t.Errorf("top = %q, want %q", out[0], want)
	}
	if want := "| body" + strings.Repeat(" ", 12) + " |"; out[1] != want {
		t.Errorf("row = %q, want %q", out[1], want)
	}
	if want := "+" + strings.Repeat("-", 18) + "+"; out[3] != want {
		t.Errorf("bottom = %q, want %q", out[3], want)
	}
}

func TestBoxTitleIsCutToFit(t *testing.T) {
	out := asciiTheme().box(boxSpec{title: "a very long account label indeed", w: 16, h: 3, padL: 1, padR: 1})
	if got := ansi.StringWidth(out[0]); got != 16 {
		t.Errorf("a long title must not widen the box: %d", got)
	}
}

func TestBoxFocusChangesTheBorderTone(t *testing.T) {
	th := theme{p: newPalette(false), g: asciiGlyphs()}
	spec := boxSpec{title: "x", rows: nil, w: 12, h: 3, padL: 1, padR: 1}
	dimmed := th.box(spec)[0]
	spec.focused = true
	focused := th.box(spec)[0]
	if !strings.HasPrefix(dimmed, "\x1b[90m+") {
		t.Errorf("an unfocused border is dim, got %q", dimmed)
	}
	if strings.HasPrefix(focused, "\x1b[90m") || !strings.HasPrefix(focused, "+") {
		t.Errorf("a focused border keeps the default foreground, got %q", focused)
	}
}

func TestBoxThumbReplacesTheRightEdge(t *testing.T) {
	spec := boxSpec{title: "x", rows: []rich{{txt("a")}, {txt("b")}, {txt("c")}}, w: 12, h: 5, padL: 1, padR: 1,
		thumb: []bool{false, true, false}}
	out := stripAll(asciiTheme().box(spec))
	if !strings.HasSuffix(out[2], "#") || strings.HasSuffix(out[1], "#") || strings.HasSuffix(out[3], "#") {
		t.Errorf("only the marked row shows the thumb: %q", out[1:4])
	}
}

func TestBottomLineHint(t *testing.T) {
	th := asciiTheme()
	if got := plainText(th.bottomLine("esc closes", 20, toneText)); got != "+- esc closes -----+" {
		t.Errorf("bottom with hint = %q", got)
	}
	if got := plainText(th.bottomLine("", 8, toneText)); got != "+------+" {
		t.Errorf("bottom without hint = %q", got)
	}
}
