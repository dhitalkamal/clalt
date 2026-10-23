package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// minWidth / minHeight are the smallest usable dimensions; below them the
// dashboard shows a single centered notice instead of a broken layout.
const (
	minWidth  = 80
	minHeight = 24
)

// View composes the persistent multi-panel frame: a header line, a body of
// stacked left panels beside a detail pane, and a context-sensitive footer.
func (m dashModel) View() string {
	if len(m.data.Accounts) == 0 {
		return "no accounts to report on\n"
	}
	w, h := m.viewWidth(), m.viewHeight()
	if w < minWidth || h < minHeight {
		return m.tooSmall(w, h)
	}
	if m.showHelp {
		return m.header() + "\n" + m.helpView(w-1) + "\n" + m.footer()
	}

	bodyH := h - 2 // header + footer
	leftW := leftColWidth(w)
	const gap = 1
	rightW := w - leftW - gap

	// left column heights: sessions short, accounts tallest.
	sessOuter := bodyH / 3
	if sessOuter < 4 {
		sessOuter = 4
	}
	acctOuter := bodyH - sessOuter
	if acctOuter < 3 {
		acctOuter = 3
	}

	leftCol := lipgloss.JoinVertical(lipgloss.Left,
		m.renderAccounts(leftW-2, acctOuter-2),
		m.renderSessions(leftW-2, sessOuter-2),
	)
	right := m.renderDetail(rightW-2, bodyH-2)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, strings.Repeat(" ", gap), right)
	return m.header() + "\n" + body + "\n" + m.footer()
}

// leftColWidth sizes the left column responsively: ~30% of the width, clamped to
// a range that keeps both the account rows and the detail pane legible.
func leftColWidth(w int) int {
	lw := w * 30 / 100
	if lw < 24 {
		lw = 24
	}
	if lw > 38 {
		lw = 38
	}
	return lw
}

// tooSmall renders the centered notice shown below the minimum size.
func (m dashModel) tooSmall(w, h int) string {
	msg := lipglossFg(colWarn).Render("terminal too small (need 80x24)")
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, msg)
}
