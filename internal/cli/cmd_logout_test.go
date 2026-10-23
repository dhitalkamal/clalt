package cli

import (
	"strings"
	"testing"
)

func TestLogoutSignsTheFolderOutAndKeepsTheEmail(t *testing.T) {
	e := newEnv(t)
	e.at(theClock)
	e.signIn("work", "a@example.com")
	folder := e.folder("work")

	code := e.run("logout", "work")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if e.claude.signedIn(folder) {
		t.Error("the folder is still signed in")
	}
	if calls := e.claude.calls("logout"); len(calls) != 1 || calls[0] != folder {
		t.Errorf("logout calls = %q, want one in the account folder", calls)
	}
	m := e.account("work").Meta
	if m.Email != "a@example.com" || m.Plan != "" || m.Org != "" || !m.SignedInAt.IsZero() {
		t.Errorf("meta = %+v: the email stays so the next login is checked, the rest goes", m)
	}
	out := e.out.String()
	for _, want := range []string{"work is signed out", "history", "clalt login work"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
}

func TestLoginAfterALogoutIsStillCheckedAgainstTheEmail(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.run("logout", "work")
	e.claude.signInAs("b@example.com")

	code := e.run("login", "work")

	if code != 1 || !strings.Contains(e.errOut.String(), "a@example.com") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
	calls := e.claude.calls("login")
	if last := calls[len(calls)-1]; !strings.HasSuffix(last, "--email a@example.com") {
		t.Errorf("the recorded email should prefill the form: %s", last)
	}
}

func TestLogoutThatFailsChangesNothing(t *testing.T) {
	e := newEnv(t)
	e.at(theClock)
	e.signIn("work", "a@example.com")
	before := e.account("work").Meta
	e.claude.set("LOGOUT_FAILS", "1")

	code := e.run("logout", "work")

	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, "could not reach the keychain") || !strings.Contains(msg, "nothing was changed") {
		t.Errorf("exit %d: %s", code, msg)
	}
	if after := e.account("work").Meta; after != before {
		t.Errorf("meta changed: %+v then %+v", before, after)
	}
	if !e.claude.signedIn(e.folder("work")) {
		t.Error("the login went away although claude said logout failed")
	}
}

func TestLogoutOfSomeoneUnknownIsAnError(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	code := e.run("logout", "nobody")

	if code != 1 || !strings.Contains(e.errOut.String(), `no account called "nobody"`) {
		t.Errorf("exit %d, stderr %q", code, e.errOut.String())
	}
	if n := len(e.claude.calls("logout")); n != 0 {
		t.Errorf("claude was asked to log out %d times", n)
	}
}

func TestLogoutMistakesAreUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"logout"}, {"logout", "a", "b"}, {"logout", "--nope"}} {
		e := newEnv(t)
		code := e.run(args...)
		if code != 2 || !strings.Contains(e.errOut.String(), "usage: clalt logout <name>") {
			t.Errorf("%v: exit %d, stderr %q", args, code, e.errOut.String())
		}
	}
}
