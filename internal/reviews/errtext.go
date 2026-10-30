package reviews

import (
	"strings"
	"unicode/utf8"
)

// errMarkerText joins the kept head and tail of a long error text.
const errMarkerText = "\n...\n"

// maxRunError caps the error text stored on a failed review.
const maxRunError = 300

// errText cleans s like tidy, then fits it in max runes. git and gh print
// progress noise first and the real cause last, so a long text keeps its
// first line and its last lines around errMarkerText instead of losing the
// end. the first line is cut to half the budget so the tail always gets room.
func errText(s string, max int) string {
	s = tidy(s, 1<<30)
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	budget := max - utf8.RuneCountInString(errMarkerText)
	if budget < 2 {
		return tidy(s, max)
	}
	runes := []rune(s)
	// head: the first line, capped at half the budget
	headLen := len(runes)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		headLen = utf8.RuneCountInString(s[:i])
	}
	if headLen > budget/2 {
		headLen = budget / 2
	}
	head := string(runes[:headLen])
	tail := string(runes[len(runes)-(budget-headLen):])
	// prefer whole trailing lines: drop a leading partial line when
	// something is left after it
	if i := strings.IndexByte(tail, '\n'); i >= 0 && i+1 < len(tail) && runes[len(runes)-utf8.RuneCountInString(tail)-1] != '\n' {
		tail = tail[i+1:]
	}
	return head + errMarkerText + tail
}
