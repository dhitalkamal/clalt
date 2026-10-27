package terminal

import "strings"

// ShellLine joins argv into one POSIX shell line, quoting only what needs it.
func ShellLine(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = shellWord(a)
	}
	return strings.Join(parts, " ")
}

func shellWord(s string) string {
	if s != "" && !strings.ContainsFunc(s, needsQuote) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func needsQuote(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return false
	}
	return !strings.ContainsRune("-_./:@+,=%", r)
}
