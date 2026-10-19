package dashboard

import (
	"strings"
	"testing"
)

func TestHalfCellRounding(t *testing.T) {
	cases := []struct {
		pct  float64
		w, n int
	}{
		{0, 10, 0}, {100, 10, 20}, {120, 10, 20}, {-5, 10, 0},
		{28, 10, 6}, {27, 10, 5}, {1, 10, 1}, {0.1, 22, 1}, {26, 22, 11}, {42, 22, 18},
	}
	for _, c := range cases {
		if got := halfCells(c.pct, c.w); got != c.n {
			t.Errorf("halfCells(%v, %d) = %d, want %d", c.pct, c.w, got, c.n)
		}
	}
}

func TestBarIsExactlyWideAndMonotonic(t *testing.T) {
	for _, g := range []glyphs{asciiGlyphs(), unicodeGlyphs()} {
		for _, w := range []int{4, 8, 14, 22, 29, 32} {
			prev := -1
			for pct := 0; pct <= 100; pct += 3 {
				b := bar(g, float64(pct), w, toneGood)
				if b.width() != w {
					t.Fatalf("bar(%d%%, %d) is %d wide", pct, w, b.width())
				}
				fill := 0
				for _, sp := range b {
					if sp.t == toneGood {
						fill += len([]rune(sp.s))
					}
				}
				if fill < prev {
					t.Fatalf("bar fill shrank from %d to %d at %d%% (w=%d)", prev, fill, pct, w)
				}
				prev = fill
			}
		}
	}
}

// fillTone is the tone of the first filled cell, or dim when nothing is filled
func fillTone(b rich) tone {
	for _, sp := range b {
		if sp.t != toneDim {
			return sp.t
		}
	}
	return toneDim
}

func TestUsageBarColorFollowsTheThresholdsAtTheirBoundaries(t *testing.T) {
	g := asciiGlyphs()
	cases := []struct {
		pct  float64
		want tone
	}{
		{0.5, toneGood}, {26, toneGood}, {69, toneGood}, {69.9, toneGood},
		{70, toneWarn}, {80, toneWarn}, {89, toneWarn}, {89.9, toneWarn},
		{90, toneBad}, {95, toneBad}, {100, toneBad},
	}
	for _, c := range cases {
		if got := fillTone(usageBar(g, c.pct, 20)); got != c.want {
			t.Errorf("usageBar(%v) fills with tone %d, want %d", c.pct, got, c.want)
		}
	}
	if got := fillTone(usageBar(g, 0, 20)); got != toneDim {
		t.Error("an empty bar is all track")
	}
}

func TestShareBarStaysNeutral(t *testing.T) {
	for _, pct := range []float64{5, 50, 95} {
		if got := fillTone(shareBar(asciiGlyphs(), pct, 12)); got != toneText {
			t.Errorf("share bar at %v%% uses tone %d, shares are not warnings", pct, got)
		}
	}
}

func TestBarCellsUseTheGlyphSet(t *testing.T) {
	g := asciiGlyphs()
	got := plainText(usageBar(g, 26, 22))
	if want := "#####" + g.half + strings.Repeat("-", 16); got != want {
		t.Errorf("26%% of 22 cells = %q, want %q", got, want)
	}
}

func TestSparkLevels(t *testing.T) {
	g := asciiGlyphs()
	got := spark(g, []float64{0, 5, 10})
	if s := got[0].s; s != g.ramp[0]+g.ramp[4]+g.ramp[7] && s != g.ramp[0]+g.ramp[3]+g.ramp[7] {
		t.Errorf("spark of 0, half, max should start low and end on the top level, got %q", s)
	}
	if s := spark(g, []float64{0, 0})[0].s; s != g.ramp[0]+g.ramp[0] {
		t.Errorf("all zero values should sit on the lowest level, got %q", s)
	}
}

func TestSparkMatchesTheApprovedWeek(t *testing.T) {
	// the seven daily costs behind the approved robin overview, oldest first
	got := spark(asciiGlyphs(), []float64{3.37, 4.05, 1.40, 0.95, 2.87, 3.62, 2.14})
	if want := "*#::+*="; got[0].s != want {
		t.Errorf("spark = %q, want %q", got[0].s, want)
	}
}
