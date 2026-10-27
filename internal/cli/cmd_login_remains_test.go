package cli

import (
	"strings"
	"testing"
)

// when a login may be left in an account's folder, the account has to be where
// clalt can sign it out: listed, and found by logout. that holds for an account
// this login made and for one that was removed and signs in again, which would
// otherwise keep a login in a folder nobody lists. each case sets the failure up
// after the account exists, since remove signs the folder out.
var loginMayBeLeft = []struct {
	name  string
	setup func(e *testEnv)
	fix   func(e *testEnv)
}{
	{"the status check breaks",
		func(e *testEnv) { e.claude.set("STATUS_BROKEN", "1") },
		func(e *testEnv) { e.claude.set("STATUS_BROKEN", "") }},
	{"an api key helper overrides the login",
		func(e *testEnv) { e.claude.set("API_KEY_HELPER", "1") },
		func(e *testEnv) { e.claude.set("API_KEY_HELPER", "") }},
	{"a provider overrides the login",
		func(e *testEnv) { e.claude.set("PROVIDER", "bedrock") },
		func(e *testEnv) { e.claude.set("PROVIDER", "") }},
	{"the sign-out after a refusal fails",
		func(e *testEnv) {
			e.signIn("other", "a@example.com")
			e.claude.signInAs("a@example.com")
			e.claude.set("LOGOUT_FAILS", "1")
		},
		func(e *testEnv) { e.claude.set("LOGOUT_FAILS", "") }},
	{"claude saves the login and then fails",
		func(e *testEnv) { e.claude.set("LOGIN_WRITES_THEN_FAILS", "1") },
		func(e *testEnv) { e.claude.set("LOGIN_WRITES_THEN_FAILS", "") }},
	{"claude fails and then cannot be asked",
		func(e *testEnv) {
			e.claude.set("LOGIN_WRITES_THEN_FAILS", "1")
			e.claude.set("STATUS_BROKEN", "1")
		},
		func(e *testEnv) {
			e.claude.set("LOGIN_WRITES_THEN_FAILS", "")
			e.claude.set("STATUS_BROKEN", "")
		}},
}

func TestAFailedLoginThatMayHaveLeftALoginKeepsTheAccountWhereItCanBeSignedOut(t *testing.T) {
	for _, kind := range []string{"a new account", "a removed account"} {
		for _, c := range loginMayBeLeft {
			t.Run(kind+", "+c.name, func(t *testing.T) {
				e := newEnv(t)
				label := "fresh"
				if kind == "a removed account" {
					label = "work"
					e.signIn(label, "b@example.com")
					if code := e.run("remove", label, "--force"); code != 0 {
						t.Fatalf("remove exited %d: %s", code, e.errOut.String())
					}
				}
				e.claude.signInAs("b@example.com")
				c.setup(e)
				e.reset()

				if code := e.run("login", label); code != 1 {
					t.Fatalf("login exit %d, want 1: %s", code, e.errOut.String())
				}

				msg := e.errOut.String()
				if e.account(label).Meta.Hidden {
					t.Error("a login may be left in the folder and the account is hidden")
				}
				if strings.Count(msg, "clalt logout "+label) != 1 {
					t.Errorf("the error should say once how to sign the folder out:\n%s", msg)
				}
				listed := false
				for _, l := range labelsInList(t, e) {
					listed = listed || l == label
				}
				if !listed {
					t.Error("the account is not in the list")
				}

				// and the way out that the error names works
				c.fix(e)
				e.reset()
				if code := e.run("logout", label); code != 0 {
					t.Fatalf("logout exit %d: %s", code, e.errOut.String())
				}
				if e.claude.signedIn(e.folder(label)) {
					t.Error("the login is still in the folder after logout")
				}
			})
		}
	}
}

// claude can fail after it saved the login, so a failed sign-in is not the same
// as a folder without one: claude is asked before the account is hidden.
func TestAFailedLoginIsAskedAboutBeforeTheAccountIsHidden(t *testing.T) {
	e := newEnv(t)
	e.claude.signInAs("b@example.com")
	e.claude.set("LOGIN_FAILS", "1")

	if code := e.run("login", "fresh"); code != 1 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if calls := e.claude.calls("status"); len(calls) != 1 {
		t.Errorf("status calls = %v, want one after the failed login", calls)
	}
	if !e.account("fresh").Meta.Hidden {
		t.Error("claude says nobody is signed in, so the account should be hidden again")
	}
	if strings.Contains(e.errOut.String(), "may be left") {
		t.Errorf("nothing is left, and the error says it may be:\n%s", e.errOut.String())
	}
}

// a removed account whose sign-in is refused, and signed out again, has nothing
// left in its folder and stays removed.
func TestARemovedAccountWhoseLoginWasRefusedStaysRemoved(t *testing.T) {
	e := newEnv(t)
	e.signIn("home", "h@example.com")
	e.signIn("work", "w@example.com")
	if code := e.run("remove", "work", "--force"); code != 0 {
		t.Fatalf("remove exited %d: %s", code, e.errOut.String())
	}
	e.reset()
	e.claude.signInAs("h@example.com")

	if code := e.run("login", "work"); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, e.errOut.String())
	}

	if m := e.account("work").Meta; !m.Hidden || m.Email != "" {
		t.Errorf("meta = %+v: the refused login must leave the account removed", m)
	}
	if e.claude.signedIn(e.folder("work")) {
		t.Error("the refused login was left in the folder")
	}
}
