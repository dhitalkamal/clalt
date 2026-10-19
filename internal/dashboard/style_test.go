package dashboard

import (
	"strings"
	"testing"
)

func TestUsageToneBoundaries(t *testing.T) {
	cases := []struct {
		pct  float64
		want tone
	}{{0, toneGood}, {69, toneGood}, {69.9, toneGood}, {70, toneWarn}, {89, toneWarn}, {89.9, toneWarn}, {90, toneBad}, {100, toneBad}}
	for _, c := range cases {
		if got := levelTone(c.pct); got != c.want {
			t.Errorf("levelTone(%v) = %v, want %v", c.pct, got, c.want)
		}
	}
	for pct, want := range map[int]tone{0: toneText, 69: toneText, 70: toneWarn, 89: toneWarn, 90: toneBad} {
		if got := pctTone(pct); got != want {
			t.Errorf("pctTone(%d) = %v, want %v", pct, got, want)
		}
	}
}

func TestLevelTones(t *testing.T) {
	if lvTone(LevelOK, toneDim) != toneDim || lvTone(LevelWarn, toneDim) != toneWarn || lvTone(LevelBad, toneDim) != toneBad {
		t.Error("a level picks its own tone, and ok keeps the base tone")
	}
}

func TestPaletteUsesOnlyTheSixteenAnsiColors(t *testing.T) {
	p := newPalette(false)
	want := map[tone]string{toneGood: "\x1b[32m", toneWarn: "\x1b[33m", toneBad: "\x1b[31m", toneDim: "\x1b[90m", toneBold: "\x1b[1m"}
	for tn, code := range want {
		if got := p.apply(tn, "x"); !strings.HasPrefix(got, code) {
			t.Errorf("tone %d renders %q, want prefix %q", tn, got, code)
		}
	}
	if got := p.apply(toneText, "x"); got != "x" {
		t.Errorf("default text should be left alone, got %q", got)
	}
	if got := p.apply(toneGood, ""); got != "" {
		t.Errorf("empty text should stay empty, got %q", got)
	}
}

func TestNoColorPaletteKeepsAttributesOnly(t *testing.T) {
	p := newPalette(true)
	allowed := map[string]bool{"": true, "0": true, "1": true, "2": true, "4": true, "22": true, "24": true}
	for tn := tone(0); tn < toneCount; tn++ {
		for _, param := range SGRParams(p.apply(tn, "x")) {
			if !allowed[param] {
				t.Errorf("tone %d emits color parameter %q with NO_COLOR", tn, param)
			}
		}
	}
	// meaning must not rest on color: warnings stay bold, dim stays faint
	for _, tn := range []tone{toneWarn, toneBad} {
		if got := p.apply(tn, "x"); !strings.Contains(got, "\x1b[1m") {
			t.Errorf("tone %d should be bold under NO_COLOR, got %q", tn, got)
		}
	}
	if got := p.apply(toneDim, "x"); !strings.Contains(got, "\x1b[2m") {
		t.Errorf("dim should be faint under NO_COLOR, got %q", got)
	}
}

func TestActiveTabIsOneEscapeNotOnePerRune(t *testing.T) {
	got := newPalette(false).apply(toneActive, "Overview")
	if n := strings.Count(got, "\x1b["); n > 2 {
		t.Errorf("underlined text should use one escape pair, found %d escapes in %q", n, got)
	}
	params := strings.Join(SGRParams(got), ";")
	if !strings.Contains(";"+params+";", ";1;") || !strings.Contains(";"+params+";", ";4;") {
		t.Errorf("the active tab should be bold and underlined, got %q", got)
	}
}

func TestDimmedPaletteMakesEveryToneDim(t *testing.T) {
	d := newPalette(false).dimmed()
	for tn := tone(0); tn < toneCount; tn++ {
		if got := d.apply(tn, "x"); !strings.HasPrefix(got, "\x1b[90m") {
			t.Errorf("dimmed tone %d renders %q", tn, got)
		}
	}
}

func TestGlyphSets(t *testing.T) {
	a, u := asciiGlyphs(), unicodeGlyphs()
	ascii := func(g glyphs) []string {
		out := []string{g.border.TopLeft, g.border.Top, g.border.Left, g.axis.Left, g.axis.BottomLeft, g.axis.MiddleBottom,
			g.thumb, g.cursor, g.dot, g.full, g.half, g.track, g.ellipsis}
		return append(out, g.ramp[:]...)
	}
	for _, s := range ascii(a) {
		if !IsASCII(s) || s == "" {
			t.Errorf("ascii glyph %q is empty or not ascii", s)
		}
	}
	for _, s := range []string{u.cursor, u.dot, u.full, u.half, u.track, u.ellipsis, u.ramp[0], u.ramp[7], u.border.TopLeft} {
		if IsASCII(s) {
			t.Errorf("unicode glyph %q is plain ascii", s)
		}
	}
	if glyphsFor(true).full != a.full || glyphsFor(false).full != u.full {
		t.Error("glyphsFor should pick the ascii set for true")
	}
	if !a.ascii || u.ascii {
		t.Error("only the ascii set is flagged ascii")
	}
	for i, r := range a.ramp {
		if i > 0 && r == a.ramp[i-1] {
			t.Errorf("ascii ramp repeats a level at %d", i)
		}
	}
}
