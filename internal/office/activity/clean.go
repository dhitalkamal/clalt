package activity

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// cutMarker ends a text that was shortened to its cap.
const cutMarker = "\n[cut]"

// cleanText strips escape sequences, control and format characters (newline
// and tab stay) and invalid utf-8, then cuts the rest to max characters. a cut
// text ends with cutMarker.
func cleanText(s string, max int) string {
	s = strings.ToValidUTF8(ansi.Strip(s), "")
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return r
		case unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp):
			return -1
		}
		return r
	}, s)
	if utf8.RuneCountInString(s) > max {
		s = string([]rune(s)[:max]) + cutMarker
	}
	return s
}
