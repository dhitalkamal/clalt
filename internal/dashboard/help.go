package dashboard

import (
	"strings"

	"charm.land/lipgloss/v2"
)

var helpRows = [][2]string{
	{"up/down, k/j", "move in the focused pane"},
	{"tab", "switch pane"},
	{"[ ] or 1-5", "switch view"},
	{"h/l, left/right", "switch history tab"},
	{"enter", "run claude as the account, or open the project when the History Projects table is focused"},
	{"s", "sign in"},
	{"e", "edit the budget"},
	{"r", "rename the account (accounts pane)"},
	{"d", "sign out the account (accounts pane)"},
	{"x", "stop the selected session"},
	{"/", "filter accounts"},
	{"a", "toggle ascii mode"},
	{"?", "show or hide this help"},
	{"esc", "close help or filter"},
	{"esc, backspace", "go back from a project"},
	{"q", "quit"},
	{"ctrl+c", "quit from anywhere"},
}

// longer descriptions wrap under themselves
const maxHelpDesc = 46

// helpLayout is the key list laid out for a screen: every line, how many of
// them the box shows at once and how the box is padded
type helpLayout struct {
	lines   []rich
	bw      int  // box width
	padX    int  // blank columns inside each side
	visible int  // lines shown at once
	roomy   bool // a blank line above and below the list
}

func (l helpLayout) maxAt() int { return max(len(l.lines)-l.visible, 0) }

func (th theme) helpLayout(w, h int) helpLayout {
	keyW, descW := 0, 0
	for _, r := range helpRows {
		keyW, descW = max(keyW, len(r[0])), max(descW, len(r[1]))
	}
	descW = min(descW, maxHelpDesc)
	padX := 2
	bw := 2 + 2*padX + keyW + 2 + descW
	if bw > w-2 {
		padX = 1
		bw = 2 + 2*padX + keyW + 2 + descW
	}
	bw = min(bw, w-2)
	var lines []rich
	for _, r := range helpRows {
		lines = append(lines, hang(rich{bold(r[0])}, keyW+2, rich{txt(r[1])}, bw-2-2*padX)...)
	}
	return helpLayout{lines: lines, bw: bw, padX: padX, visible: min(len(lines), h-2), roomy: h >= len(lines)+6}
}

// helpBox is the keys box for a screen of w by h. a screen too short for every
// row shows the rows from line at on, with a thumb on the right edge.
func (th theme) helpBox(w, h, at int) []string {
	l := th.helpLayout(w, h)
	at = clampInt(at, 0, l.maxAt())
	rows := append([]rich(nil), l.lines[at:at+l.visible]...)
	hint := "esc closes"
	var thumb []bool
	if l.maxAt() > 0 {
		hint = "up/down scrolls, esc closes"
		thumb = thumbRows(len(l.lines), l.visible, at)
	}
	if l.roomy {
		rows = append(append([]rich{nil}, rows...), nil)
	}
	return th.box(boxSpec{title: "Keys", rows: rows, w: l.bw, h: len(rows) + 2, padL: l.padX, padR: l.padX, focused: true, hint: hint, thumb: thumb})
}

// overlay draws box centered over the frame lines. the compositor trims
// trailing spaces, so short lines are fine and missing ones are added.
func overlay(base, box []string, w, h int) []string {
	bw := lipgloss.Width(box[0])
	// blank gutter so backdrop text does not touch the box
	if w >= bw+4 {
		gutter := make([]string, len(box))
		for i, l := range box {
			gutter[i] = " " + l + " "
		}
		box, bw = gutter, bw+2
	}
	x := max((w-bw)/2, 0)
	y := max((h-len(box))/2, 0)
	under := lipgloss.NewLayer(strings.Join(base, "\n"))
	over := lipgloss.NewLayer(strings.Join(box, "\n")).X(x).Y(y).Z(1)
	lines := strings.Split(lipgloss.NewCompositor(under, over).Render(), "\n")
	for len(lines) < h {
		lines = append(lines, "")
	}
	return lines[:h]
}
