package dashboard

import "testing"

func TestGeometryBreakpoints(t *testing.T) {
	cases := []struct {
		w, h         int
		detail       bool
		mode         layoutMode
		left, right  int
		paneH, bodyW int
	}{
		{59, 40, false, modeTooSmall, 0, 0, 38, 0},
		{200, 15, false, modeTooSmall, 0, 0, 13, 0},
		{60, 16, false, modeSingle, 58, 0, 14, 0},
		{60, 16, true, modeSingle, 0, 58, 14, 54},
		{71, 20, true, modeSingle, 0, 69, 18, 65},
		{72, 20, false, modeNarrow, 22, 47, 18, 43},
		{80, 24, false, modeNarrow, 22, 55, 22, 51},
		{95, 30, false, modeNarrow, 22, 70, 28, 66},
		{96, 30, false, modeWide, 26, 67, 28, 63},
		{100, 30, false, modeWide, 26, 71, 28, 67},
		{140, 40, true, modeWide, 26, 111, 38, 107},
	}
	for _, c := range cases {
		g := geometry(c.w, c.h, c.detail)
		if g.mode != c.mode || g.leftW != c.left || g.rightW != c.right || g.paneH != c.paneH || g.bodyW != c.bodyW {
			t.Errorf("geometry(%d, %d, detail=%v) = %+v, want mode %d left %d right %d paneH %d bodyW %d",
				c.w, c.h, c.detail, g, c.mode, c.left, c.right, c.paneH, c.bodyW)
		}
	}
	if g := geometry(100, 30, false); g.viewH != g.paneH-4 {
		t.Errorf("the view sits under the tab row and a blank line: viewH %d, paneH %d", g.viewH, g.paneH)
	}
}

func TestPanesTileTheWidth(t *testing.T) {
	for w := 72; w <= 200; w++ {
		g := geometry(w, 30, false)
		if used := margin + g.leftW + 1 + g.rightW + margin; used != w {
			t.Fatalf("w=%d: margins, panes and the gutter use %d columns", w, used)
		}
	}
}

func TestShrunkTakesLinesFromThePanes(t *testing.T) {
	g := geometry(100, 30, false)
	s := g.shrunk(1)
	if s.paneH != g.paneH-1 || s.viewH != g.viewH-1 || s.leftW != g.leftW || s.rightW != g.rightW {
		t.Errorf("one line less for the panes, nothing else: %+v from %+v", s, g)
	}
	if small := geometry(50, 14, false).shrunk(1); small.mode != modeTooSmall {
		t.Errorf("the too small screen stays too small: %+v", small)
	}
}
