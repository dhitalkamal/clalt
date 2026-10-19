package dashboard

import (
	"errors"
	"strings"
	"testing"
)

func TestFirstLineIsTheFirstOneWithAnyText(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"one line", "disk full", "disk full"},
		{"second line", "first\nsecond", "first"},
		{"leading blank lines", "\n\n  \t\nreal reason\nmore", "real reason"},
		{"windows line ends", "\r\nreal reason\r\nmore", "real reason"},
		{"trims the line", "   padded   \nnext", "padded"},
		{"nothing but blanks", "\n \n\t\n", ""},
		{"empty", "", ""},
		{"color codes", "\x1b[31mred reason\x1b[0m\nnext", "red reason"},
		{"a link", "see \x1b]8;;http://x.example\x07docs\x1b]8;;\x07 now", "see docs now"},
		{"a code that is the whole first line", "\x1b[2K\nreal reason", "real reason"},
		{"control characters", "a\tb\x07c", "a b c"},
	}
	for _, c := range cases {
		if got := firstLine(c.in); got != c.want {
			t.Errorf("%s: firstLine(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestTheHeaderShowsTheFirstLineThatSaysSomething(t *testing.T) {
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, testOptions(), 100, 30)
	m, _ = upd(m, loadedMsg{err: errors.New("\n\n\x1b[31mreal reason\x1b[0m\nsecond")})
	header := strings.Split(Screen(m), "\n")[0]
	if !strings.Contains(header, "update failed: real reason") || strings.Contains(header, "second") || strings.Contains(header, "\x1b") {
		t.Errorf("one clean line: %q", header)
	}
}

func TestAnActionErrorShowsItsFirstLineThatSaysSomething(t *testing.T) {
	m, _ := actionModel(&fakeActions{err: errors.New("\nkeychain is locked\nunlock it first")})
	if got := footerOf(pressSettle(m, "d", "y")); got != "  keychain is locked" {
		t.Errorf("flash = %q", got)
	}
}

func TestAFlashTooLongForTheScreenEndsInAnEllipsis(t *testing.T) {
	long := strings.Repeat("a reason that goes on and on ", 10)
	for w := 60; w <= 120; w += 10 {
		m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, testOptions(), w, 30)
		m, _ = m.say(long, true)
		foot := footerOf(m)
		if len(foot) != w || !strings.HasSuffix(foot, "...") || !strings.HasPrefix(foot, "  a reason that goes on") {
			t.Errorf("w=%d: the flash keeps its start and ends in an ellipsis: %q", w, foot)
		}
	}
	opt := testOptions()
	opt.ASCII = false
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, opt, 60, 30)
	m, _ = m.say(long, true)
	if foot := footerOf(m); !strings.HasSuffix(foot, cp(0x2026)) {
		t.Errorf("unicode mode ends with the real ellipsis: %q", foot)
	}
	short := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, testOptions(), 60, 30)
	short, _ = short.say("fits", false)
	if footerOf(short) != "  fits" {
		t.Errorf("a short flash is left alone: %q", footerOf(short))
	}
}
