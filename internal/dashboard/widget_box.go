package dashboard

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// theme is the palette and glyph set a frame is drawn with
type theme struct {
	p palette
	g glyphs
}

// boxSpec describes one titled pane. thumb marks the inner rows whose right
// edge shows the scroll thumb, hint is the text in the bottom border.
type boxSpec struct {
	title      string
	rows       []rich
	w, h       int
	padL, padR int
	focused    bool
	thumb      []bool
	hint       string
}

// lipgloss borders have no titles, so the top line is composed by hand
func (th theme) topLine(title string, w int, bt, tt tone) rich {
	b := th.g.border
	title = ansi.Truncate(title, max(w-5, 0), "")
	fill := max(w-5-ansi.StringWidth(title), 0)
	return rich{
		span{bt, b.TopLeft + b.Top + " "},
		span{tt, title},
		span{bt, " " + strings.Repeat(b.Top, fill) + b.TopRight},
	}
}

func (th theme) bottomLine(hint string, w int, bt tone) rich {
	b := th.g.border
	if hint == "" {
		return rich{span{bt, b.BottomLeft + strings.Repeat(b.Bottom, max(w-2, 0)) + b.BottomRight}}
	}
	hint = ansi.Truncate(hint, max(w-5, 0), "")
	fill := max(w-5-ansi.StringWidth(hint), 0)
	return rich{
		span{bt, b.BottomLeft + b.Bottom + " "},
		span{toneDim, hint},
		span{bt, " " + strings.Repeat(b.Bottom, fill) + b.BottomRight},
	}
}

// box draws a pane exactly w by h cells
func (th theme) box(s boxSpec) []string {
	bt, tt := toneDim, toneText
	if s.focused {
		bt, tt = toneText, toneBold
	}
	b := th.g.border
	cw := max(s.w-2-s.padL-s.padR, 0)
	out := make([]string, 0, s.h)
	out = append(out, th.p.draw(th.topLine(s.title, s.w, bt, tt)))
	for i := 0; i < s.h-2; i++ {
		var row rich
		if i < len(s.rows) {
			row = s.rows[i]
		}
		edge, et := b.Right, bt
		if i < len(s.thumb) && s.thumb[i] {
			edge, et = th.g.thumb, toneText
		}
		line := cat(rich{span{bt, b.Left}, gap(s.padL)}, row.fit(cw), rich{gap(s.padR), span{et, edge}})
		out = append(out, th.p.draw(line))
	}
	return append(out, th.p.draw(th.bottomLine(s.hint, s.w, bt)))
}
