package dashboard

import (
	"fmt"
	"strings"
)

const (
	minW, minH = 60, 16 // below this the dashboard does not render
	twoPaneW   = 72     // below this only one pane shows at a time
	widePaneW  = 96     // from here the accounts pane is 26 wide, else 22
	margin     = 1      // blank columns left and right of the panes
	leftNarrow = 22
	leftWide   = 26
)

type layoutMode int

const (
	modeTooSmall layoutMode = iota
	modeSingle
	modeNarrow
	modeWide
)

type geom struct {
	mode          layoutMode
	leftW, rightW int // outer widths, 0 when a pane is hidden
	paneH         int // outer pane height
	viewH         int // rows under the tab row and its blank line
	bodyW         int // text width inside the right pane
}

// geometry sizes the panes. with one pane showing, detail picks which one.
func geometry(w, h int, detail bool) geom {
	g := geom{paneH: h - 2}
	g.viewH = g.paneH - 4
	avail := w - 2*margin
	switch {
	case w < minW || h < minH:
		g.mode = modeTooSmall
	case w < twoPaneW:
		g.mode = modeSingle
		if detail {
			g.rightW = avail
		} else {
			g.leftW = avail
		}
	case w < widePaneW:
		g.mode = modeNarrow
		g.leftW = leftNarrow
		g.rightW = avail - g.leftW - 1
	default:
		g.mode = modeWide
		g.leftW = leftWide
		g.rightW = avail - g.leftW - 1
	}
	g.bodyW = max(g.rightW-4, 0)
	return g
}

// shrunk takes n lines from the panes, for lines between the header and them
func (g geom) shrunk(n int) geom {
	if g.mode != modeTooSmall {
		g.paneH -= n
		g.viewH -= n
	}
	return g
}

// tooSmall is the whole screen when the terminal is under 60 by 16
func (th theme) tooSmall(w, h int) string {
	cw := max(w-2, 10)
	body := wrap(rich{txt(fmt.Sprintf("Terminal too small. Make it at least %d by %d.", minW, minH))}, cw)
	body = append(body, wrap(rich{dim(fmt.Sprintf("Current size is %d by %d.", w, h))}, cw)...)
	top := max((h-len(body))/2, 0)
	out := make([]string, h)
	for i := range out {
		if j := i - top; j >= 0 && j < len(body) {
			l := body[j].clip(w)
			out[i] = th.p.draw(cat(rich{gap((w - l.width()) / 2)}, l))
		}
	}
	return strings.Join(out, "\n")
}
