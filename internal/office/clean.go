package office

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// text caps, in characters. a label is a short description, the rest are names.
const (
	maxName  = 60
	maxLabel = 80
)

// clean makes a string safe for the snapshot: escape sequences are removed,
// control, format and line separator characters become spaces, runs of spaces
// collapse, and the result is cut to max characters. the page sets it as text,
// never as markup, but the snapshot should be clean on its own.
func clean(s string, max int) string {
	s = strings.ToValidUTF8(ansi.Strip(s), "")
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > max {
		s = string([]rune(s)[:max])
	}
	return strings.TrimSpace(s)
}

// folderName is the last folder of a working directory, with either kind of
// separator. the rest of the path is never shown.
func folderName(cwd string) string {
	p := strings.TrimRight(strings.ReplaceAll(cwd, `\`, "/"), "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		p = p[i+1:]
	}
	return p
}

// noEmail keeps an email address out of the snapshot: a value with an at sign
// is cut there, and one that starts with it becomes "account".
func noEmail(s string) string {
	if i := strings.Index(s, "@"); i >= 0 {
		s = strings.TrimSpace(s[:i])
		if s == "" {
			return "account"
		}
	}
	return s
}
