package cli

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// clean makes a name safe to print on a terminal. control and format
// characters could forge lines or send escape sequences, so they show as
// escapes instead. text that claude reports and names found on disk go through
// it before they are stored or printed.
func clean(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, `\x%02x`, s[i])
		case unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp) && !isJoiner(r):
			b.WriteString(escape(r))
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

// isJoiner is true for the zero width joiner and non-joiner. they are format
// characters, but scripts and emoji sequences need them, and they can neither
// move the cursor nor start a line.
func isJoiner(r rune) bool { return r == '\u200c' || r == '\u200d' }

func escape(r rune) string {
	switch {
	case r < 0x100:
		return fmt.Sprintf(`\x%02x`, r)
	case r < 0x10000:
		return fmt.Sprintf(`\u%04x`, r)
	}
	return fmt.Sprintf(`\U%08x`, r)
}

func cleanErr(err error) error { return errors.New(clean(err.Error())) }
