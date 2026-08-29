package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

// emitJSON writes v as indented JSON to the app's output.
func (a *App) emitJSON(v any) error {
	enc := json.NewEncoder(a.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// printf writes a formatted human line to the app's output.
func (a *App) printf(format string, args ...any) {
	fmt.Fprintf(a.Out, format, args...)
}

// prompt reads one trimmed line after showing a message. It reuses a single
// buffered reader so read-ahead does not swallow input meant for a later prompt.
func (a *App) prompt(msg string) (string, error) {
	fmt.Fprint(a.Out, msg)
	if a.reader == nil {
		a.reader = bufio.NewReader(a.In)
	}
	line, err := a.reader.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// confirm asks a yes/no question, defaulting to no.
func (a *App) confirm(msg string) bool {
	ans, _ := a.prompt(msg + " [y/N]: ")
	ans = strings.ToLower(ans)
	return ans == "y" || ans == "yes"
}

// promptPassword reads a passphrase without echoing it. Falls back to a normal
// line read when stdin is not a terminal (e.g. piped input in tests).
func (a *App) promptPassword(msg string) (string, error) {
	fmt.Fprint(a.Out, msg)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(a.Out)
		return strings.TrimSpace(string(b)), err
	}
	return a.prompt("")
}

// humanDuration renders a signed duration until t as a compact "in 4h12m" /
// "expired 3m ago" string.
func humanUntil(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Until(t)
	if d < 0 {
		return "expired " + compactDur(-d) + " ago"
	}
	return "in " + compactDur(d)
}

func compactDur(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 24 {
		return fmt.Sprintf("%dd%dh", h/24, h%24)
	}
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
