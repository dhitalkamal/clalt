package dashboard

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// tones are roles, not colors: only the 16 ANSI colors are used so the
// terminal theme picks the shades. good is green for healthy usage and
// working, warn is yellow for close and waiting, bad is red for the limit, dim
// is secondary text and empty tracks, active is the selected tab.
type tone int

const (
	toneText tone = iota
	toneDim
	toneGood
	toneWarn
	toneBad
	toneBold
	toneActive
	toneCount
)

// with NO_COLOR the palette keeps bold, dim and underline but emits no color
type palette struct {
	paint [toneCount]func(string) string
}

func painter(st lipgloss.Style) func(string) string {
	return func(s string) string { return st.Render(s) }
}

func fg(c ansi.BasicColor) func(string) string {
	return painter(lipgloss.NewStyle().Foreground(c))
}

func newPalette(noColor bool) palette {
	var p palette
	plain := func(s string) string { return s }
	bold := painter(lipgloss.NewStyle().Bold(true))
	p.paint[toneText] = plain
	p.paint[toneBold] = bold
	// lipgloss v2 wraps each rune of underlined text in its own escape, so the
	// active tab goes through x/ansi for one clean sequence
	p.paint[toneActive] = func(s string) string { return ansi.NewStyle().Bold().Underline(true).Styled(s) }
	if noColor {
		p.paint[toneDim] = painter(lipgloss.NewStyle().Faint(true))
		p.paint[toneGood] = plain
		p.paint[toneWarn] = bold
		p.paint[toneBad] = bold
		return p
	}
	p.paint[toneDim] = fg(lipgloss.BrightBlack)
	p.paint[toneGood] = fg(lipgloss.Green)
	p.paint[toneWarn] = fg(lipgloss.Yellow)
	p.paint[toneBad] = fg(lipgloss.Red)
	return p
}

// dimmed is the backdrop under the help overlay
func (p palette) dimmed() palette {
	var d palette
	for i := range d.paint {
		d.paint[i] = p.paint[toneDim]
	}
	return d
}

func (p palette) apply(t tone, s string) string {
	if s == "" {
		return ""
	}
	return p.paint[t](s)
}

// usage colors: green under 70, yellow 70 to 89, red 90 and up
func levelTone(pct float64) tone {
	switch {
	case pct >= 90:
		return toneBad
	case pct >= 70:
		return toneWarn
	}
	return toneGood
}

// numbers only get a color when they need attention
func pctTone(pct int) tone {
	switch {
	case pct >= 90:
		return toneBad
	case pct >= 70:
		return toneWarn
	}
	return toneText
}

func lvTone(lv Level, base tone) tone {
	switch lv {
	case LevelBad:
		return toneBad
	case LevelWarn:
		return toneWarn
	}
	return base
}
