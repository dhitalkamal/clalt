package cli

import (
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// the recorded email is compared ignoring case, like every other email check
func TestLoginAcceptsTheRecordedEmailInAnotherCase(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.claude.signInAs("A@Example.COM")

	if code := e.run("login", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
}

// claude too old to drive means nobody signed in, and claude was never asked to
func TestAFirstLoginWithAClaudeThatIsTooOldLeavesNoAccountInTheList(t *testing.T) {
	e := newEnv(t)
	e.claude.set("VERSION", "2.0.0")

	if code := e.run("login", "fresh"); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, e.errOut.String())
	}

	if !e.account("fresh").Meta.Hidden {
		t.Error("the account is in the list")
	}
	if len(e.claude.calls("login")) != 0 {
		t.Error("claude was asked to sign in")
	}
}

// what claude says on stderr reaches the user
func TestLoginShowsWhatClaudeSaysOnStderr(t *testing.T) {
	e := newEnv(t)
	e.claude.set("LOGIN_FAILS", "1")

	e.run("login", "work")

	if !strings.Contains(e.errOut.String(), "login was cancelled") {
		t.Errorf("claude's own explanation is lost:\n%s", e.errOut.String())
	}
}

// the refusal says why even when the folder could not be signed out again
func TestTheForcedSignOutFailureStillNamesTheEmails(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.claude.signInAs("b@example.com")
	e.claude.set("LOGOUT_FAILS", "1")

	e.run("login", "work")

	msg := e.errOut.String()
	if !strings.Contains(msg, "a@example.com") || !strings.Contains(msg, "b@example.com") {
		t.Errorf("the refusal does not name the emails: %s", msg)
	}
}

// a refusal clears org and plan too, not only the time of the sign-in, and
// keeps the email so that the next sign-in is still checked
func TestARefusalClearsOrgAndPlanAndKeepsTheEmail(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.claude.signInAs("b@example.com")

	e.run("login", "work")

	if m := e.account("work").Meta; m.Org != "" || m.Plan != "" || !m.SignedInAt.IsZero() || m.Email != "a@example.com" {
		t.Errorf("meta = %+v", m)
	}
}

// a new label never takes over a folder that is already there
func TestANewLabelNeverLandsOnAFolderThatIsAlreadyThere(t *testing.T) {
	e := newEnv(t)
	orphan := accountDir(t, "work")
	e.claude.plantLogin(orphan, "someone@example.com")
	e.claude.signInAs("a@example.com")

	if code := e.run("login", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := e.account("work").Name; got != "work-2" {
		t.Errorf("id = %q, want work-2: the new account took over a folder that is not its own", got)
	}
	if !e.claude.signedIn(orphan) {
		t.Error("the folder that was already there lost its login")
	}
}

// a folder that cannot be looked at is never taken for a free one
func TestLoginStopsWhenTheAccountsFolderCannotBeLookedAt(t *testing.T) {
	e := newEnv(t)
	mustMkdir(t, config.AccountsDir())
	if !makeUnlookable(t, config.AccountsDir()) {
		t.Skip("no permission bits to take away")
	}

	if code := e.run("login", "work"); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, e.errOut.String())
	}

	if e.hasAccount("work") {
		t.Error("an account was made for an id whose folder could not be checked")
	}
	noClaudeCalls(t, e)
}

// a status that names another folder is not a sign-in to this one
func TestLoginRefusesAStatusFromAnotherFolder(t *testing.T) {
	e := newEnv(t)
	e.claude.signInAs("a@example.com")
	e.claude.set("STATUS_FOLDER", "/somewhere/else")

	code := e.run("login", "work")

	if code != 1 || !strings.Contains(e.errOut.String(), "a different folder") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
	if m := e.account("work").Meta; m.Email != "" || !m.SignedInAt.IsZero() {
		t.Errorf("an identity was recorded: %+v", m)
	}
}

// the warning is about the folder, which the sync looked at, so it is printed
// once whatever becomes of the sign-in: a login that did not work still has it,
// next to its own error
func TestTheHistoryWarningIsPrintedOnceWhateverBecomesOfTheSignIn(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *testEnv)
		code  int
	}{
		{"it works", func(e *testEnv) {}, 0},
		{"claude exits zero and signs nobody in", func(e *testEnv) { e.claude.set("LOGIN_NOOP", "1") }, 1},
		{"an api key helper overrides the login", func(e *testEnv) { e.claude.set("API_KEY_HELPER", "1") }, 1},
		{"the email belongs to another account", func(e *testEnv) { e.claude.signInAs("a@example.com") }, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.seedShared()
			e.signIn("other", "a@example.com")
			unmountHistory(t, e)
			e.claude.signInAs("b@example.com")
			c.setup(e)

			if code := e.run("login", "fresh"); code != c.code {
				t.Fatalf("exit %d, want %d: %s", code, c.code, e.errOut.String())
			}

			if n := strings.Count(e.errOut.String(), "will refuse"); n != 1 {
				t.Errorf("the warning was printed %d times:\n%s", n, e.errOut.String())
			}
		})
	}
}
