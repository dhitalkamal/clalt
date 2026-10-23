package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// a renamed account has a label that is not its id: lookups and messages use
// the label, the store and the folder use the id.
func TestRenameOfARenamedAccountFindsItByItsLabel(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	if code := e.run("rename", "work", "office"); code != 0 {
		t.Fatalf("rename: exit %d: %s", code, e.errOut.String())
	}
	e.reset()

	code := e.run("rename", "office", "desk")

	if code != 0 || !strings.Contains(e.out.String(), "renamed office to desk") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, e.out.String(), e.errOut.String())
	}
	if acc := e.account("desk"); acc.Name != "work" {
		t.Errorf("id = %q, want work", acc.Name)
	}
	if err := e.app.Rename("desk", "den"); err != nil {
		t.Fatalf("the dashboard renames by label too: %v", err)
	}
	if acc := e.account("den"); acc.Name != "work" {
		t.Errorf("id = %q, want work", acc.Name)
	}
}

func TestLogoutAndRemoveSpeakInLabelsOnceAnAccountIsRenamed(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.run("rename", "work", "office")
	e.reset()

	if code := e.run("logout", "office"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if out := e.out.String(); !strings.Contains(out, "office is signed out") || !strings.Contains(out, "clalt login office") {
		t.Errorf("logout output: %q", out)
	}
	e.signIn("office", "w@example.com")
	e.claude.set("LOGOUT_FAILS", "1")
	for _, args := range [][]string{{"logout", "office"}, {"remove", "office", "--force"}} {
		e.reset()
		if code := e.run(args...); code != 1 || !strings.Contains(e.errOut.String(), "could not sign office out") {
			t.Errorf("%v: exit %d: %s", args, code, e.errOut.String())
		}
	}
	e.claude.set("LOGOUT_FAILS", "")
	e.reset()
	if code := e.run("remove", "office", "--force"); code != 0 || !strings.Contains(e.out.String(), "removed office") || !strings.Contains(e.out.String(), "clalt login office") {
		t.Errorf("remove: exit %d, stdout %q", code, e.out.String())
	}
}

func TestLogoutKeepsTheFolderAndTheHistoryLink(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	folder := e.folder("work")

	if code := e.run("logout", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if !exists(folder) || !exists(filepath.Join(folder, "projects")) {
		t.Error("logout must keep the folder and its history link")
	}
}

func TestRemoveTakesTheShortForceFlag(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	if code := e.run("remove", "-f", "work"); code != 0 || !e.account("work").Meta.Hidden {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}

func TestRemoveThatIsAbortedRunsNoClaudeAndNamesTheAccount(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	before := len(e.claude.calls("status"))
	e.answer("n\n")

	e.run("remove", "work")

	if got := len(e.claude.calls("status")); got != before {
		t.Errorf("claude was asked %d more times before the user said yes", got-before)
	}
	if out := e.out.String(); !strings.Contains(out, "remove work?") || !strings.Contains(out, "[y/N]") {
		t.Errorf("the question is missing or does not name the account: %q", out)
	}
}

func TestRemoveWhoseSignOutIsUnknownKeepsTheWholeRecordAndSaysHowToCheck(t *testing.T) {
	e := newEnv(t)
	e.at(theClock)
	e.signIn("work", "w@example.com")
	before := e.account("work").Meta
	e.claude.set("LOGOUT_HANGS", "1")
	old := authTimeout
	authTimeout = 300 * time.Millisecond
	t.Cleanup(func() { authTimeout = old })

	code := e.run("remove", "work", "--force")

	msg := e.errOut.String()
	for _, want := range []string{"the account was not removed", "may be signed out already", "clalt list", "clalt remove work"} {
		if code != 1 || !strings.Contains(msg, want) {
			t.Errorf("exit %d, missing %q: %s", code, want, msg)
		}
	}
	if after := e.account("work").Meta; after != before {
		t.Errorf("the record changed: %+v then %+v", before, after)
	}
}

func TestDoctorDoesNotAskClaudeAboutTheFolderOfAnUnsafeId(t *testing.T) {
	e := newEnv(t)
	plantState(t, unsafeState)
	// the folder the id would point at, with a login in it
	e.claude.plantLogin(filepath.Join(e.home, ".config", "escape"), "x@example.com")

	doctor(t, e)

	noClaudeCalls(t, e)
}

func TestControllerLogoutOfARemovedOrUnknownAccountFails(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	if err := e.app.Remove("work"); err != nil {
		t.Fatal(err)
	}
	before := len(e.claude.calls("logout"))

	for _, label := range []string{"work", "nobody"} {
		if err := e.app.Logout(label); err == nil {
			t.Errorf("Logout(%q) should fail, like clalt logout", label)
		}
	}
	if got := len(e.claude.calls("logout")); got != before {
		t.Error("claude was asked to sign a removed account out")
	}
}

// a folder that is already there is never adopted: it may hold somebody's login
func TestLoginUsesANumberedIdWhenAFolderIsAlreadyThere(t *testing.T) {
	e := newEnv(t)
	// a folder with a login and no account, as a lost or edited state file leaves it
	e.claude.plantLogin(accountDir(t, "work"), "old@example.com")
	e.claude.signInAs("new@example.com")

	if code := e.run("login", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if acc := e.account("work"); acc.Name != "work-2" {
		t.Errorf("id = %q, want work-2", acc.Name)
	}
	body, _ := os.ReadFile(filepath.Join(accountDir(t, "work"), ".fake-login"))
	if !strings.HasPrefix(string(body), "old@example.com") {
		t.Errorf("the folder that was already there was used: %q", body)
	}
}
