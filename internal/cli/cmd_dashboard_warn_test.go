package cli

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// what the dashboard could not load is a warning on a dashboard that still opens
// and still lists the accounts: an empty list of sessions that is really a
// folder that cannot be read must not read as "no running sessions", and usage
// that could not be read must not read as zero.
func TestTheDashboardDataSaysWhenTheSessionsCannotBeRead(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	liveSession(t, "work", "/code/x")
	if !makeUnlookable(t, config.SessionsDir()) {
		t.Skip("no permission bits to take away")
	}

	data, err := e.app.Data()

	if err != nil {
		t.Fatalf("Data: %v", err)
	}
	if !strings.Contains(data.Warning, "sessions") || !strings.Contains(data.Warning, "permission denied") {
		t.Errorf("Warning = %q, want it to say the sessions could not be read and why", data.Warning)
	}
	if len(data.Accounts) != 1 {
		t.Errorf("accounts = %+v: the dashboard still lists them", data.Accounts)
	}
}

func TestTheDashboardDataSaysWhenTheUsageCannotBeRead(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	unmountHistory(t, e)

	data, err := e.app.Data()

	if err != nil {
		t.Fatalf("Data: %v", err)
	}
	if !strings.Contains(data.Warning, "usage") || !strings.Contains(data.Warning, "leads nowhere") {
		t.Errorf("Warning = %q, want it to say the usage could not be read and why", data.Warning)
	}
	if len(data.Accounts) != 1 || data.Accounts[0].Name != "work" {
		t.Errorf("accounts = %+v", data.Accounts)
	}
	frame, err := e.app.dashboardFrame()
	if err != nil || !strings.Contains(frame, "usage could not be read") {
		t.Errorf("the static frame does not show the warning (%v):\n%s", err, frame)
	}
}

func TestTheDashboardDataHasNoWarningWhenAllIsRead(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	liveSession(t, "work", "/code/x")

	data, err := e.app.Data()

	if err != nil || data.Warning != "" {
		t.Errorf("Data = warning %q, %v", data.Warning, err)
	}
}

func TestBothWarningsAreKept(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	liveSession(t, "work", "/code/x")
	unmountHistory(t, e)
	if !makeUnlookable(t, config.SessionsDir()) {
		t.Skip("no permission bits to take away")
	}

	data, err := e.app.Data()

	if err != nil || !strings.Contains(data.Warning, "usage") || !strings.Contains(data.Warning, "sessions") {
		t.Errorf("Warning = %q, %v", data.Warning, err)
	}
}

// the state file is the one thing the dashboard cannot do without: no accounts
// means nothing to show, and that is an error
func TestTheDashboardDataStillFailsWhenTheStateCannotBeRead(t *testing.T) {
	e := newEnv(t)
	plantState(t, "{not json")

	if _, err := e.app.Data(); err == nil {
		t.Error("Data accepted a state file that cannot be read")
	}
}

// the errors the dashboard shows come from claude and from the file system, and
// the dashboard draws them: they leave this package clean
func TestControllerErrorsAreCleanedForTheDashboard(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.set("LOGOUT_FAILS", "1")
	e.claude.set("LOGOUT_ERR", `bad \033]0;PWNED\007 \033[2Jline\nforged`)

	for name, call := range map[string]func() error{
		"logout": func() error { return e.app.Logout("work") },
		"remove": func() error { return e.app.Remove("work") },
		"rename": func() error { return e.app.Rename("work", "bad\x1b[31mname") },
		"budget": func() error { return e.app.SetBudget("nobody\x1b[31m", 1, 1) },
	} {
		err := call()
		if err == nil || hasControl(err.Error()) {
			t.Errorf("%s: err = %q", name, err)
		}
	}
}

func TestControllerDataErrorIsCleanedToo(t *testing.T) {
	e := newEnv(t)
	plantState(t, "{not json \x1b[31m")

	_, err := e.app.Data()

	if err == nil || hasControl(err.Error()) {
		t.Errorf("err = %q", err)
	}
}

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

// a session records the folder it was started in, and a folder can be named
// anything: the dashboard draws it
func TestTheDashboardDataCleansTheFolderOfASession(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	liveSession(t, "work", "/code/x\x1b]0;owned\x07\nforged")

	data, err := e.app.Data()

	if err != nil || len(data.Sessions) != 1 || hasControl(data.Sessions[0].Cwd) {
		t.Errorf("sessions = %+v, %v", data.Sessions, err)
	}
}

// an error that names a path names the one the user has, and that can hold
// anything too
func TestControllerErrorsThatNameAPathAreCleaned(t *testing.T) {
	e := newEnv(t)
	hostile := filepath.Join(e.home, "x\x1b[31mred")
	t.Setenv("XDG_CONFIG_HOME", hostile)
	plantState(t, `{"active":"","accounts":{"work":{"addedAt":"2026-01-01T00:00:00Z","label":"work"}}}`)
	app := NewApp()
	if !makeReadOnly(t, config.ClaltDir()) {
		t.Skip("no permission bits to take away")
	}

	for name, call := range map[string]func() error{
		"rename": func() error { return app.Rename("work", "other") },
		"budget": func() error { return app.SetBudget("work", 1, 1) },
	} {
		if err := call(); err == nil || hasControl(err.Error()) || !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("%s: err = %q, want a permission error without control characters", name, err)
		}
	}
}
