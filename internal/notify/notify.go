// Package notify tells the user that a claude session needs them, through the
// desktop notification of the machine.
package notify

import (
	"context"
	"strings"
	"unicode"

	"github.com/dhitalkamal/clalt/internal/runcmd"
)

// maxText is the longest title or text, in characters.
const maxText = 120

// Note is one notification.
type Note struct {
	Title string
	Text  string
}

// Notifier shows a Note.
type Notifier interface {
	Notify(ctx context.Context, n Note) error
}

// System returns the notifier of this machine, or false when there is none.
func System(r runcmd.Runner, goos string) (Notifier, bool) {
	switch {
	case goos == "darwin":
		return macNotifier{r}, true
	case goos == "linux" && r.Has("notify-send"):
		return linuxNotifier{r}, true
	}
	return nil, false
}

type macNotifier struct{ r runcmd.Runner }

// Notify passes title and text as arguments of the script, never as source.
func (m macNotifier) Notify(ctx context.Context, n Note) error {
	_, err := m.r.Run(ctx, "osascript",
		"-e", "on run argv",
		"-e", "display notification (item 3 of argv) with title (item 1 of argv) subtitle (item 2 of argv)",
		"-e", "end run",
		"clalt", noDash(clean(n.Title, maxText)), noDash(clean(n.Text, maxText)))
	return err
}

type linuxNotifier struct{ r runcmd.Runner }

// Notify ends the options with -- so no text can be read as one.
func (l linuxNotifier) Notify(ctx context.Context, n Note) error {
	// notify-send runs the body through g_strcompress, and servers read markup
	body := strings.NewReplacer(`\`, `\\`, "&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(clean(n.Text, maxText))
	_, err := l.r.Run(ctx, "notify-send", "--app-name=clalt", "--", clean(n.Title, maxText), body)
	return err
}

// noDash keeps a leading dash from looking like an option.
func noDash(s string) string {
	if strings.HasPrefix(s, "-") {
		return " " + s
	}
	return s
}

// clean makes s one line of at most max characters. control characters become
// spaces and runs of white space shrink to one space. an empty result is
// "clalt", so the title is never empty.
func clean(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.ToValidUTF8(s, "")
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > max {
		s = strings.TrimSpace(string(r[:max]))
	}
	if s == "" {
		return "clalt"
	}
	return s
}
