package cli

import (
	"errors"
	"strings"
	"testing"
)

// a refused run or login prints its reason, and the dashboard opens again at
// once on top of it. the reason has to stay readable until the user has seen it.
func TestAFailedDashboardActionIsShownAndWaitsForEnter(t *testing.T) {
	for _, verb := range []string{"run", "login"} {
		t.Run(verb, func(t *testing.T) {
			e := newEnv(t)
			e.answer("\n")

			e.app.afterDashAction(errors.New("work is not ready to run: not signed in"), verb)

			if msg := e.errOut.String(); !strings.Contains(msg, verb+" failed: work is not ready to run") {
				t.Errorf("stderr = %q", msg)
			}
			if out := e.out.String(); !strings.Contains(out, "enter") {
				t.Errorf("stdout = %q: it should say what it waits for", out)
			}
			if rest, _ := e.app.In.(interface{ Len() int }); rest != nil && rest.Len() != 0 {
				t.Error("it did not read the line it waited for")
			}
		})
	}
}

func TestADashboardActionThatWorkedDoesNotWait(t *testing.T) {
	e := newEnv(t)
	e.answer("\n")

	e.app.afterDashAction(nil, "run")
	e.app.afterDashAction(exitError{3}, "run") // a session that ended with a code is claude's business

	if e.errOut.Len() != 0 || e.out.Len() != 0 {
		t.Errorf("stdout %q, stderr %q: nothing to say", e.out.String(), e.errOut.String())
	}
	if rest := e.app.In.(interface{ Len() int }).Len(); rest != 1 {
		t.Error("it waited for a line although nothing failed")
	}
}

// without a terminal on stdin there is nobody to press enter, and the dashboard
// does not hang
func TestAFailedDashboardActionDoesNotHangWithoutInput(t *testing.T) {
	e := newEnv(t)
	e.answer("")

	e.app.afterDashAction(errors.New("boom"), "run")

	if !strings.Contains(e.errOut.String(), "run failed: boom") {
		t.Errorf("stderr = %q", e.errOut.String())
	}
}

func TestAFailedDashboardActionIsCleaned(t *testing.T) {
	e := newEnv(t)
	e.answer("\n")

	e.app.afterDashAction(errors.New("bad \x1b[31mred\nforged"), "login")

	if hasControl(e.errOut.String()) {
		t.Errorf("stderr = %q", e.errOut.String())
	}
}
