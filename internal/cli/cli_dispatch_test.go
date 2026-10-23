package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/version"
)

// version, help and the short names reach their commands, and a bare --json is
// no command at all
func TestVersionHelpAndTheShortNamesReachTheirCommands(t *testing.T) {
	e := newEnv(t)
	for _, arg := range []string{"version", "--version", "-v"} {
		e.reset()
		if code := e.run(arg); code != 0 || strings.TrimSpace(e.out.String()) != version.String() {
			t.Errorf("%s: exit %d, printed %q, want %q", arg, code, e.out.String(), version.String())
		}
	}
	for _, arg := range []string{"help", "-h", "--help"} {
		e.reset()
		if code := e.run(arg); code != 0 || !strings.Contains(e.out.String(), "usage:") {
			t.Errorf("%s: exit %d, printed %q", arg, code, e.out.String())
		}
	}
	e.reset()
	if code := e.run("--json"); code != 0 || !strings.Contains(e.out.String(), "usage:") {
		t.Errorf("--json alone: exit %d, printed %q", code, e.out.String())
	}
	for _, c := range [][2]string{{"ls", "list"}, {"current", "whoami"}} {
		e.reset()
		e.run(c[0])
		alias := e.out.String()
		e.reset()
		e.run(c[1])
		if alias == "" || alias != e.out.String() {
			t.Errorf("%s printed %q, %s printed %q", c[0], alias, c[1], e.out.String())
		}
	}
}

func TestAnUnknownCommandSaysWhereToLook(t *testing.T) {
	e := newEnv(t)
	if code := e.run("frobnicate"); code != 2 || e.errOut.String() != "unknown command \"frobnicate\" (try 'clalt help')\n" {
		t.Errorf("exit %d: %q", code, e.errOut.String())
	}
}

// the mistake in a command line is printed back, and what was typed can carry
// anything
func TestUsageErrorsAreCleaned(t *testing.T) {
	rlo := string(rune(0x202e))
	for _, args := range [][]string{{"run", "--\x1b[2Jx"}, {"logout", "--\x1b]0;x\x07"}, {"login", "-" + rlo + "x"}} {
		e := newEnv(t)
		if code := e.run(args...); code != 2 {
			t.Fatalf("%q exit %d", args, code)
		}
		if got := e.errOut.String(); hasControl(got) || strings.Contains(got, rlo) || !strings.Contains(got, "unknown flag") {
			t.Errorf("%q printed %q", args, got)
		}
	}
}

// the error of a run or login that the dashboard started is printed before the
// dashboard comes back, and it comes from claude and from the disk
func TestTheErrorOfADashboardActionIsCleaned(t *testing.T) {
	e := newEnv(t)

	e.app.afterDashAction(errors.New("boom \x1b[2J"), "run")

	if got := e.errOut.String(); hasControl(got) || !strings.Contains(got, "run failed: boom") {
		t.Errorf("printed %q", got)
	}
}
