package main

import "strings"

// percentB expands some of the backslash escapes that printf %b does: \n, \t,
// \r, \a, \\ and \0NNN octal, which is how the tests spell an escape
// character. any other backslash pair is left as it is.
func percentB(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch c := s[i]; c {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'a':
			b.WriteByte(7)
		case '\\':
			b.WriteByte('\\')
		case '0':
			j, v := i+1, 0
			for ; j < len(s) && j < i+4 && s[j] >= '0' && s[j] <= '7'; j++ {
				v = v*8 + int(s[j]-'0')
			}
			b.WriteByte(byte(v))
			i = j - 1
		default:
			b.WriteByte('\\')
			b.WriteByte(c)
		}
	}
	return b.String()
}

// rawQuote wraps s in quotes with the backslash and quote escaped, for a path:
// on windows every path has backslashes.
func rawQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
