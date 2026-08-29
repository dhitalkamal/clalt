package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// box-drawing glyphs, built from rune values so no literal box chars sit in source.
const (
	bxTL = 0x250C
	bxTR = 0x2510
	bxBL = 0x2514
	bxBR = 0x2518
	bxH  = 0x2500
	bxV  = 0x2502
)

func repRune(r rune, n int) string {
	if n < 0 {
		n = 0
	}
	return strings.Repeat(string(r), n)
}

// titledBox wraps body (which may contain ANSI color) in a titled border. innerW
// is the content width between the vertical borders; the title is inlaid into the
// top border. Content lines are padded (ANSI-aware) so the right border aligns.
func titledBox(title string, innerW int, body string) string {
	label := " " + title + " "
	if lipgloss.Width(label) > innerW {
		label = label[:innerW]
	}
	top := borderStyle.Render(string(rune(bxTL))+string(rune(bxH))) +
		sectionStyle.Render(label) +
		borderStyle.Render(repRune(bxH, innerW-lipgloss.Width(label)-1)+string(rune(bxTR)))
	bottom := borderStyle.Render(string(rune(bxBL)) + repRune(bxH, innerW) + string(rune(bxBR)))

	v := borderStyle.Render(string(rune(bxV)))
	var b strings.Builder
	b.WriteString(top + "\n")
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		pad := innerW - lipgloss.Width(line)
		if pad < 0 {
			pad = 0
		}
		b.WriteString(v + line + strings.Repeat(" ", pad) + v + "\n")
	}
	b.WriteString(bottom)
	return b.String()
}
