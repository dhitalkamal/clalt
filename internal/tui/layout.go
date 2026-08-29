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

// pluralAccounts renders "1 account" / "3 accounts".
func pluralAccounts(n int) string {
	if n == 1 {
		return "1 account"
	}
	return itoa(n) + " accounts"
}

func itoa(n int) string { return strconv.Itoa(n) }

// labeledValue renders "label  value" with the label dimmed, padded to labelW.
func labeledValue(label, value string, labelW int) string {
	return dimStyle.Render(padRight(label, labelW)) + value
}

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
