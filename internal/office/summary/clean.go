package summary

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// clean makes transcript text safe to send: escapes removed, control and
// format characters become spaces, runs of spaces collapse, cut to max runes.
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

// lastFolder is the last folder of a path with either kind of separator.
func lastFolder(p string) string {
	p = strings.TrimRight(strings.ReplaceAll(p, `\`, "/"), "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		p = p[i+1:]
	}
	return p
}

// relTo shows file relative to dir when it is inside it, else as given.
func relTo(file, dir string) string {
	f := strings.ReplaceAll(file, `\`, "/")
	d := strings.TrimRight(strings.ReplaceAll(dir, `\`, "/"), "/")
	if d != "" {
		if rest, ok := strings.CutPrefix(f, d+"/"); ok && rest != "" {
			return rest
		}
	}
	return f
}
