package tui

import (
	"strings"
	"time"
)

// renderSessions draws the live `clalt run` sessions panel. When focused, the
// selected session is highlighted and k kills it.
func (m dashModel) renderSessions(innerW, innerH int) string {
	focused := m.focus == focusSessions
	var b strings.Builder
	if len(m.data.Sessions) == 0 {
		b.WriteString(dimStyle.Render(trunc("no running sessions", innerW)))
	}
	start, end := windowRange(m.sessSel, len(m.data.Sessions), innerH)
	for i := start; i < end; i++ {
		b.WriteString(m.sessionRow(m.data.Sessions[i], i == m.sessSel, focused, innerW) + "\n")
	}
	return panelBox("Sessions", innerW, innerH, b.String(), focused)
}

// sessionRow formats one session line: a liveness dot, account, pid, and uptime.
func (m dashModel) sessionRow(s SessionInfo, selected, focused bool, innerW int) string {
	cur := "  "
	if selected {
		cur = cursorStyle.Render("> ")
	}
	dot := okStyle.Render(string(rune(0x25CF)))
	info := s.Account + " " + dimStyle.Render(itoa(s.PID)+" "+sessUptime(s.Started))
	row := cur + dot + " " + clampLine(info, innerW-5)
	if selected && focused {
		row = rowSelStyle.Render(padRight(clampLine(row, innerW), innerW))
	}
	return row
}

func sessUptime(start time.Time) string {
	if start.IsZero() {
		return "-"
	}
	d := time.Since(start).Round(time.Minute)
	h := int(d.Hours())
	mn := int(d.Minutes()) % 60
	if h > 0 {
		return itoa(h) + "h" + itoa(mn) + "m"
	}
	return itoa(mn) + "m"
}
