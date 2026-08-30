package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// spinFrames is a subtle braille spinner shown in the header while a background
// refresh is in flight. Built from rune values so no glyph literals sit in source.
var spinFrames = []string{
	string(rune(0x280B)), string(rune(0x2819)), string(rune(0x2839)), string(rune(0x2838)),
	string(rune(0x283C)), string(rune(0x2834)), string(rune(0x2826)), string(rune(0x2827)),
	string(rune(0x2807)), string(rune(0x280F)),
}

// panelBox renders body inside a titled border of an exact inner size (innerW x
// innerH), with the title inlaid into the top border. A focused panel gets a
// bright accent border and title; others are dimmed. Content is padded or
// truncated (ANSI-aware) so every panel keeps a stable footprint for the layout.
func panelBox(title string, innerW, innerH int, body string, focused bool) string {
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	bstyle := borderStyle
	tstyle := panelTitleDim
	if focused {
		bstyle = lipglossFg(colAccent)
		tstyle = sectionStyle
	}

	label := " " + title + " "
	if lipgloss.Width(label) > innerW {
		label = clampLine(label, innerW)
	}
	top := bstyle.Render(string(rune(bxTL))+string(rune(bxH))) +
		tstyle.Render(label) +
		bstyle.Render(repRune(bxH, innerW-lipgloss.Width(label)-1)+string(rune(bxTR)))
	bottom := bstyle.Render(string(rune(bxBL)) + repRune(bxH, innerW) + string(rune(bxBR)))
	v := bstyle.Render(string(rune(bxV)))

	var b strings.Builder
	b.WriteString(top + "\n")
	for _, line := range splitToHeight(body, innerW, innerH) {
		b.WriteString(v + line + v + "\n")
	}
	b.WriteString(bottom)
	return b.String()
}

// splitToHeight normalizes body into exactly innerH lines, each padded (ANSI
// aware) to innerW; extra lines are dropped and short bodies padded with blanks.
func splitToHeight(body string, innerW, innerH int) []string {
	raw := strings.Split(strings.TrimRight(body, "\n"), "\n")
	blank := strings.Repeat(" ", innerW)
	out := make([]string, 0, innerH)
	for _, line := range raw {
		if len(out) >= innerH {
			break
		}
		line = clampLine(line, innerW)
		pad := innerW - lipgloss.Width(line)
		if pad < 0 {
			pad = 0
		}
		out = append(out, line+strings.Repeat(" ", pad))
	}
	for len(out) < innerH {
		out = append(out, blank)
	}
	return out
}

// clampLine truncates a (possibly styled) line to w display columns.
func clampLine(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(w).Render(s)
}
