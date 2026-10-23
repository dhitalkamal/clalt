package cli

import (
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// claude changes the folder first and clalt records it after. when the record
// cannot be saved, the folder is already signed out or in, and an error that only
// says "permission denied" leaves the user to guess which. these tests take away
// the right to write in clalt's folder after the accounts are made.
func accountsWithAStateThatCannotBeSaved(t *testing.T) *testEnv {
	t.Helper()
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.signIn("other", "o@example.com")
	if !makeReadOnly(t, config.ClaltDir()) {
		t.Skip("no permission bits to take away")
	}
	e.reset()
	return e
}

func TestLogoutThatCannotSaveItsRecordSaysTheFolderIsSignedOut(t *testing.T) {
	e := accountsWithAStateThatCannotBeSaved(t)

	code := e.run("logout", "work")

	msg := e.errOut.String()
	for _, want := range []string{"work is signed out", "could not save that", "permission denied", "'clalt logout work' tries again"} {
		if code != 1 || !strings.Contains(msg, want) {
			t.Errorf("exit %d, stderr is missing %q:\n%s", code, want, msg)
		}
	}
	if e.claude.signedIn(e.folder("work")) {
		t.Error("the folder is still signed in, and the message says it is not")
	}
	if strings.Contains(e.out.String(), "is signed out") {
		t.Errorf("the success line was printed:\n%s", e.out.String())
	}
}

func TestRemoveThatCannotSaveItsRecordSaysTheFolderIsSignedOut(t *testing.T) {
	e := accountsWithAStateThatCannotBeSaved(t)

	code := e.run("remove", "work", "--force")

	msg := e.errOut.String()
	for _, want := range []string{"work is signed out", "not removed", "could not save that", "'clalt remove work' tries again"} {
		if code != 1 || !strings.Contains(msg, want) {
			t.Errorf("exit %d, stderr is missing %q:\n%s", code, want, msg)
		}
	}
	if e.claude.signedIn(e.folder("work")) {
		t.Error("the folder is still signed in, and the message says it is not")
	}
	if e.account("work").Meta.Hidden {
		t.Error("the account was hidden although the file could not be written")
	}
}

// nothing was signed out when the folder was not signed in, and the message
// must not say that something was
func TestRemoveOfASignedOutAccountThatCannotSaveItsRecordDoesNotClaimASignOut(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	if code := e.run("logout", "work"); code != 0 {
		t.Fatalf("logout exited %d: %s", code, e.errOut.String())
	}
	if !makeReadOnly(t, config.ClaltDir()) {
		t.Skip("no permission bits to take away")
	}
	e.reset()

	code := e.run("remove", "work", "--force")

	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, "permission denied") || strings.Contains(msg, "is signed out") {
		t.Errorf("exit %d, stderr:\n%s", code, msg)
	}
	if len(e.claude.calls("logout")) != 1 {
		t.Errorf("logout calls = %v: remove had nothing to sign out", e.claude.calls("logout"))
	}
}

// the refusal is why the folder was signed out again, and that stays the first
// thing the message says even when the record of it cannot be saved
func TestARefusedLoginThatCannotSaveItsRecordKeepsItsReason(t *testing.T) {
	e := accountsWithAStateThatCannotBeSaved(t)
	e.claude.signInAs("o@example.com")

	code := e.run("login", "work")

	msg := e.errOut.String()
	for _, want := range []string{`"work" is set up for w@example.com, but the sign-in was o@example.com`, "signed out again", "could not save that"} {
		if code != 1 || !strings.Contains(msg, want) {
			t.Errorf("exit %d, stderr is missing %q:\n%s", code, want, msg)
		}
	}
	if e.claude.signedIn(e.folder("work")) {
		t.Error("the wrong person's login was left in the folder")
	}
}

func TestALoginThatCannotSaveItsRecordSaysTheFolderIsSignedIn(t *testing.T) {
	e := accountsWithAStateThatCannotBeSaved(t)
	e.claude.signInAs("w@example.com")

	code := e.run("login", "work")

	msg := e.errOut.String()
	for _, want := range []string{"the sign-in worked", "could not record it", "'clalt login work' again records it"} {
		if code != 1 || !strings.Contains(msg, want) {
			t.Errorf("exit %d, stderr is missing %q:\n%s", code, want, msg)
		}
	}
	if !e.claude.signedIn(e.folder("work")) {
		t.Error("the folder is not signed in, and the message says it is")
	}
}
