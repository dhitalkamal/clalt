package tui

import (
	"fmt"
	"strings"
	"time"
)

// viewSessions lists live `clalt run` sessions (parallel isolated profiles).
func (m dashModel) viewSessions() string {
	innerW := m.viewWidth() - 2
	if len(m.data.Sessions) == 0 {
		body := dimStyle.Render("no running sessions") + "\n" +
			dimStyle.Render("start one with ") + infoStyle.Render("clalt run <account>") +
			dimStyle.Render(" - it runs in an isolated profile, in parallel")
		return titledBox("PARALLEL SESSIONS", innerW, body)
	}
	var b strings.Builder
	b.WriteString(dimStyle.Render(
		padRight("  account", 14)+padRight("pid", 9)+padRight("uptime", 9)+"cwd") + "\n")
	for _, s := range m.data.Sessions {
		b.WriteString(okStyle.Render(string(rune(0x25CF))+" ") +
			padRight(s.Account, 12) +
			padRight(fmt.Sprintf("%d", s.PID), 9) +
			padRight(sessUptime(s.Started), 9) +
			dimStyle.Render(s.Cwd) + "\n")
	}
	return titledBox(fmt.Sprintf("PARALLEL SESSIONS  %d running", len(m.data.Sessions)), innerW, b.String())
}

func sessUptime(start time.Time) string {
	if start.IsZero() {
		return "-"
	}
	d := time.Since(start).Round(time.Minute)
	h := int(d.Hours())
	mn := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, mn)
	}
	return fmt.Sprintf("%dm", mn)
}
