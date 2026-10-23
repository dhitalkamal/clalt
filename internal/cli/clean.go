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
// escapes instead.
func clean(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, `\x%02x`, s[i])
		case unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp):
			b.WriteString(escape(r))
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

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
