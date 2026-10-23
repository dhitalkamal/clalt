// Package tui holds the interactive terminal views: the account picker used by
// bare `clalt switch` and the full-screen usage dashboard.
package tui

import "github.com/charmbracelet/lipgloss"

// Semantic color slots. A cyan/magenta identity (distinct from clauth's orange)
// on a Tokyo-Night base. Hex downsamples cleanly on 256-color and is exact on
// truecolor, so the hierarchy survives everywhere.
var (
	colBrand  = lipgloss.Color("#2ac3de") // logo mark (teal)
	colAccent = lipgloss.Color("#7dcfff") // panel titles, active tab, primary bars (cyan)
	colBar    = lipgloss.Color("#7dcfff") // primary chart bars
	colBar2   = lipgloss.Color("#c099ff") // secondary bars (magenta)
	colCost   = lipgloss.Color("#c099ff") // cost figures (magenta)
	colOK     = lipgloss.Color("#9ece6a") // green: healthy / active
	colWarn   = lipgloss.Color("#e0af68") // yellow: caution
	colErr    = lipgloss.Color("#f7768e") // red: near/over limit
	colInfo   = lipgloss.Color("#2ac3de") // teal accents
	colText   = lipgloss.Color("#c8d3f5")
	colDim    = lipgloss.Color("#5b638f")
	colBorder = lipgloss.Color("#3a3f5c") // panel borders
	colInk    = lipgloss.Color("#0b0c12") // text on a filled tab
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(colBrand)
	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	cursorStyle  = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	dimStyle     = lipgloss.NewStyle().Foreground(colDim)
	textStyle    = lipgloss.NewStyle().Foreground(colText)
	barStyle     = lipgloss.NewStyle().Foreground(colBar)
	costStyle    = lipgloss.NewStyle().Foreground(colCost)
	okStyle      = lipgloss.NewStyle().Foreground(colOK)
	infoStyle    = lipgloss.NewStyle().Foreground(colInfo)
	borderStyle  = lipgloss.NewStyle().Foreground(colBorder)
	sparkStyle   = lipgloss.NewStyle().Foreground(colBar2)

	panelTitleDim = lipgloss.NewStyle().Bold(true).Foreground(colDim)

	rowSelStyle = lipgloss.NewStyle().Background(lipgloss.Color("#243250"))
	helpKey     = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
)

// lipglossFg is a tiny helper for foreground-only styles.
func lipglossFg(c lipgloss.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

// gradedColor maps a 0..1 utilization to a semantic color: green under half,
// yellow approaching the limit, red near or over it. Never color-only: callers
// also print the percent.
func gradedColor(frac float64) lipgloss.Color {
	switch {
	case frac >= 0.8:
		return colErr
	case frac >= 0.5:
		return colWarn
	default:
		return colOK
	}
}
