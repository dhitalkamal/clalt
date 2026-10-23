package cli

import (
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// a removed account has been signed out, and nobody lists it. a login can be
// left in its folder all the same, so doctor looks at those folders: one status
// call for each that exists, a warning when claude still reports a claude.ai
// login there, and nothing at all when all is well.
func removedAccount(t *testing.T) (*testEnv, string) {
	t.Helper()
	e := newEnv(t)
	e.seedShared()
	e.signIn("home", "h@example.com")
	e.signIn("work", "w@example.com")
	if code := e.run("remove", "work", "--force"); code != 0 {
		t.Fatalf("remove exited %d: %s", code, e.errOut.String())
	}
	e.reset()
	return e, e.folder("work")
}

func statusCallsFor(e *testEnv, folder string) int {
	n := 0
	for _, call := range e.claude.calls("status") {
		if call == folder {
			n++
		}
	}
	return n
}

func TestDoctorWarnsAboutARemovedAccountWhoseFolderIsStillSignedIn(t *testing.T) {
	e, folder := removedAccount(t)
	e.claude.plantLogin(folder, "w@example.com")
	before := statusCallsFor(e, folder)

	code, rep := doctor(t, e)

	rep.want(t, "work", "removedLogin", "warn", "removed but its folder is still signed in")
	rep.want(t, "work", "removedLogin", "warn", "clalt logout work")
	if code != 0 || rep.Failed != 0 || rep.Warned != 1 {
		t.Errorf("exit %d, %d failed, %d warned, want 0, 0, 1: %+v", code, rep.Failed, rep.Warned, rep.Checks)
	}
	if got := statusCallsFor(e, folder) - before; got != 1 {
		t.Errorf("status calls for the removed account = %d, want one", got)
	}
}

func TestDoctorSaysNothingAboutARemovedAccountThatIsSignedOut(t *testing.T) {
	e, folder := removedAccount(t)
	before := statusCallsFor(e, folder)

	code, rep := doctor(t, e)

	for _, c := range rep.Checks {
		if c.Scope == "work" {
			t.Errorf("a check for a removed account that is fine: %+v", c)
		}
	}
	if code != 0 || rep.Warned != 0 {
		t.Errorf("exit %d, %d warned: %+v", code, rep.Warned, rep.Checks)
	}
	if got := statusCallsFor(e, folder) - before; got != 1 {
		t.Errorf("status calls for the removed account = %d, want one", got)
	}
}

func TestDoctorDoesNotAskAboutARemovedAccountWithoutAFolder(t *testing.T) {
	e := newEnv(t)
	plantState(t, `{"active":"","accounts":{"gone":{"addedAt":"2026-01-01T00:00:00Z","hidden":true}}}`)

	code, rep := doctor(t, e)

	if code != 0 {
		t.Errorf("exit %d: %+v", code, rep.Checks)
	}
	for _, c := range rep.Checks {
		if c.Scope == "gone" {
			t.Errorf("a check for a removed account with no folder: %+v", c)
		}
	}
	noClaudeCalls(t, e)
}

// an id that cannot be a folder name has no folder to look at
func TestDoctorSkipsARemovedAccountWhoseIdCannotBeAFolderName(t *testing.T) {
	e := newEnv(t)
	plantState(t, `{"active":"","accounts":{"../../escape":{"addedAt":"2026-01-01T00:00:00Z","hidden":true}}}`)

	code, rep := doctor(t, e)

	if code != 0 || rep.Failed != 0 {
		t.Errorf("exit %d, %d failed: %+v", code, rep.Failed, rep.Checks)
	}
	noClaudeCalls(t, e)
	noFolderOutside(t, e)
}

// what claude reports for a removed account that is not a claude.ai login is not
// something logout can remove, so it is not something to warn about
func TestDoctorLeavesAnOverrideAloneForARemovedAccount(t *testing.T) {
	e, _ := removedAccount(t)
	e.claude.set("API_KEY_HELPER", "1")

	_, rep := doctor(t, e)

	if _, ok := rep.find("work", "removedLogin"); ok {
		t.Errorf("a login that logout cannot remove was reported: %+v", rep.Checks)
	}
}

func TestDoctorSaysWhenItCannotAskAboutARemovedAccount(t *testing.T) {
	e, _ := removedAccount(t)
	e.claude.set("STATUS_BROKEN", "1")

	code, rep := doctor(t, e)

	rep.want(t, "work", "removedLogin", "warn", "could not check")
	rep.want(t, "work", "removedLogin", "warn", "keychain is locked")
	if code != 0 {
		t.Errorf("exit %d: not knowing is a warning", code)
	}
}

func TestDoctorDoesNotAskAboutRemovedAccountsWhenClaudeCannotBeUsed(t *testing.T) {
	e, folder := removedAccount(t)
	e.claude.plantLogin(folder, "w@example.com")
	e.claude.set("VERSION", "2.0.0")
	before := statusCallsFor(e, folder)

	_, rep := doctor(t, e)

	if _, ok := rep.find("work", "removedLogin"); ok {
		t.Errorf("a removed account was checked with a claude that is too old: %+v", rep.Checks)
	}
	if got := statusCallsFor(e, folder); got != before {
		t.Errorf("claude was asked: status calls %d then %d", before, got)
	}
}

func TestDoctorTellsRemovedAccountsApartByTheirLabels(t *testing.T) {
	e, folder := removedAccount(t)
	e.claude.plantLogin(folder, "w@example.com")
	if code := e.run("rename", "work", "old-work"); code != 0 {
		t.Fatalf("rename exited %d: %s", code, e.errOut.String())
	}
	e.reset()

	_, rep := doctor(t, e)

	rep.want(t, "old-work", "removedLogin", "warn", "clalt logout old-work")
	if c, ok := rep.find("old-work", "removedLogin"); ok && strings.Contains(c.Detail, "clalt logout work") {
		t.Errorf("the advice names the id and not the label: %q", c.Detail)
	}
}

// a removed account and a visible one whose ids differ only in case are one
// folder on a file system that ignores case, and the login there is the visible
// account's. logging out the removed one would sign that out, so doctor does not
// advise it, and does not ask claude about a folder that is not the removed
// account's.
func TestDoctorDoesNotBlameARemovedAccountForTheLoginOfOneThatSharesItsFolder(t *testing.T) {
	e := newEnv(t)
	plantState(t, `{"active":"","accounts":{`+
		`"Robin":{"email":"k@example.com","addedAt":"2026-01-01T00:00:00Z","hidden":true},`+
		`"robin":{"email":"k2@example.com","addedAt":"2026-01-02T00:00:00Z"}}}`)
	forceCollision(t, true)
	var removedFolder string
	for _, id := range []string{"Robin", "robin"} {
		folder, err := config.AccountDirFor(id)
		if err != nil {
			t.Fatal(err)
		}
		if id == "Robin" {
			removedFolder = folder
		}
		e.claude.plantLogin(folder, "k2@example.com")
	}
	before := statusCallsFor(e, removedFolder)

	_, rep := doctor(t, e)

	if c, ok := rep.find("Robin", "removedLogin"); ok {
		t.Errorf("the removed account is blamed for the login of the one that shares its folder: %+v", c)
	}
	if got := statusCallsFor(e, removedFolder) - before; got != 0 {
		t.Errorf("claude was asked %d times about the folder of the removed account", got)
	}
}
