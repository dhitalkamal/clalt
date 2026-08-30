package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// padBetween places left and right on one line, width wide, with the gap between
// filled by spaces. If they do not fit, it just joins them with a space.
func padBetween(left, right string, width int) string {
	lw := lipgloss.Width(left)
	rw := lipgloss.Width(right)
	gap := width - lw - rw
	if gap < 1 {
		return left + " " + right
	}
	return left + strings.Repeat(" ", gap) + right
}

func itoa(n int) string { return strconv.Itoa(n) }

func padRight(s string, w int) string {
	if lipgloss.Width(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}

func padLeft(s string, w int) string {
	if lipgloss.Width(s) >= w {
		return s
	}
	return strings.Repeat(" ", w-lipgloss.Width(s)) + s
}
