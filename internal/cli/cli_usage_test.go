package cli

import (
	"strings"
	"testing"
)

func TestHelpListsTheCurrentCommandsAndNoRetiredOnes(t *testing.T) {
	e := newEnv(t)

	if code := e.run("help"); code != 0 {
		t.Fatalf("exit %d", code)
	}

	out := e.out.String()
	for _, want := range []string{
		"clalt login <name>", "clalt run <name>", "clalt logout <name>", "clalt remove <name>",
		"clalt rename <old> <new>", "clalt list", "clalt whoami", "clalt doctor", "clalt migrate",
		"clalt sessions", "clalt budget", "clalt usage [name]", "clalt dashboard", "clalt dashboard --demo", "clalt version",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the help is missing %q:\n%s", want, out)
		}
	}
	for _, gone := range []string{"clalt add", "clalt refresh", "clalt keepalive", "clalt export", "clalt import", "--live", "--fix", "vault", "active one marked", "keychain-free"} {
		if strings.Contains(out, gone) {
			t.Errorf("the help still mentions %q:\n%s", gone, out)
		}
	}
}

func TestNoArgumentsShowsTheHelp(t *testing.T) {
	e := newEnv(t)
	if code := e.run(); code != 0 || !strings.Contains(e.out.String(), "usage:") {
		t.Errorf("exit %d:\n%s", code, e.out.String())
	}
}

func TestUnknownCommandIsAUsageError(t *testing.T) {
	e := newEnv(t)
	if code := e.run("frobnicate"); code != 2 || !strings.Contains(e.errOut.String(), "unknown command") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}

func TestHelpIsPlainAscii(t *testing.T) {
	e := newEnv(t)
	e.run("help")
	for i, r := range e.out.String() {
		if r > 126 {
			t.Fatalf("non-ascii character %q at byte %d", r, i)
		}
	}
}
