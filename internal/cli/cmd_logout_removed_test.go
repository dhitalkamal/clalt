package cli

import (
	"strings"
	"testing"
)

// a removed account has been signed out, but a login can be left in its folder
// all the same: a sign-in that failed after claude saved it, or one made behind
// clalt's back. logout works on it, so that nothing is left that cannot be
// signed out.
func removedAccountWithALoginLeft(t *testing.T) (*testEnv, string) {
	t.Helper()
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	if code := e.run("remove", "work", "--force"); code != 0 {
		t.Fatalf("remove exited %d: %s", code, e.errOut.String())
	}
	folder := e.folder("work")
	e.claude.plantLogin(folder, "w@example.com")
	e.reset()
	return e, folder
}

func TestLogoutSignsOutARemovedAccountAndLeavesItRemoved(t *testing.T) {
	e, folder := removedAccountWithALoginLeft(t)

	code := e.run("logout", "work")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if e.claude.signedIn(folder) {
		t.Error("the login is still in the folder")
	}
	if m := e.account("work").Meta; !m.Hidden {
		t.Errorf("meta = %+v: logout must not bring a removed account back", m)
	}
	out := e.out.String()
	if !strings.Contains(out, "work is signed out") || !strings.Contains(out, "stays removed") || !strings.Contains(out, "clalt login work") {
		t.Errorf("output:\n%s", out)
	}
	if strings.Contains(out, "its history stays. sign in again") {
		t.Errorf("a removed account is not told to sign in again as if it were listed:\n%s", out)
	}
}

func TestLogoutOfARemovedAccountThatCannotBeSignedOutChangesNothing(t *testing.T) {
	e, folder := removedAccountWithALoginLeft(t)
	e.claude.set("LOGOUT_FAILS", "1")

	code := e.run("logout", "work")

	if code != 1 || !strings.Contains(e.errOut.String(), "nothing was changed") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
	if !e.claude.signedIn(folder) {
		t.Error("the login is gone although the sign-out failed")
	}
	if !e.account("work").Meta.Hidden {
		t.Error("the account came back")
	}
}

func TestLogoutOfARemovedAccountWithoutAFolderNeedsNoClaude(t *testing.T) {
	e := newEnv(t)
	plantState(t, `{"active":"","accounts":{"gone":{"addedAt":"2026-01-01T00:00:00Z","hidden":true}}}`)

	code := e.run("logout", "gone")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	noClaudeCalls(t, e)
}
