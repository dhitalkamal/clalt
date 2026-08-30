package tui

import "strings"

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
