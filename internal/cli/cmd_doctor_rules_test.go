package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/account"
)

func TestDoctorSaysTheRulesAreFine(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	e.setRules("work", account.Rules{Folders: []string{subfolder(t, "a"), subfolder(t, "b")}})

	code, rep := doctor(t, e)

	rep.want(t, "work", "rules", "ok", "2 folders, auto-switch off")
	if code != 0 || rep.Failed != 0 || rep.Warned != 0 {
		t.Errorf("exit %d, %d failed, %d warned: %+v", code, rep.Failed, rep.Warned, rep.Checks)
	}
}

func TestDoctorCountsOneFolderInTheSingular(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	e.setRules("work", account.Rules{Folders: []string{subfolder(t, "a")}, AutoSwitch: true})

	_, rep := doctor(t, e)

	rep.want(t, "work", "rules", "ok", "1 folder, auto-switch on")
}

func TestDoctorWarnsAboutAFolderThatIsGone(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	gone := filepath.Join(t.TempDir(), "deleted")
	e.setRules("work", account.Rules{Folders: []string{subfolder(t, "there"), gone}})

	code, rep := doctor(t, e)

	rep.want(t, "work", "rules", "warn", gone)
	rep.want(t, "work", "rules", "warn", "does not exist")
	rep.want(t, "work", "rules", "warn", "clalt rules work --remove-folder")
	// a folder that is gone is something the user fixes, not something broken
	if code != 0 || rep.Failed != 0 {
		t.Errorf("exit %d, %d failed: %+v", code, rep.Failed, rep.Checks)
	}
}

func TestDoctorWarnsAboutARuleThatIsNotAnAbsolutePath(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	e.setRules("work", account.Rules{Folders: []string{"projects/work"}})

	_, rep := doctor(t, e)

	rep.want(t, "work", "rules", "warn", "projects/work")
	rep.want(t, "work", "rules", "warn", "is not an absolute path")
}

func TestDoctorSaysTheAccountServesAnyFolderWhenItIsOnlyAutoSwitch(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	e.setRules("work", account.Rules{AutoSwitch: true})

	_, rep := doctor(t, e)

	rep.want(t, "work", "rules", "ok", "no folder rules")
	rep.want(t, "work", "rules", "ok", "auto-switch on")
}

// an account nobody set rules for has nothing to check, and the report stays as
// it was
func TestDoctorHasNoRulesLineForAnAccountWithoutRules(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")

	_, rep := doctor(t, e)

	if c, ok := rep.find("work", "rules"); ok {
		t.Errorf("a rules check for an account with no rules: %+v", c)
	}
}

func TestDoctorLeavesTheRulesOfARemovedAccountAlone(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("gone", "g@example.com")
	e.setRules("gone", account.Rules{Folders: []string{filepath.Join(t.TempDir(), "deleted")}})
	if code := e.run("remove", "gone", "--force"); code != 0 {
		t.Fatalf("remove exited %d: %s", code, e.errOut.String())
	}

	_, rep := doctor(t, e)

	if c, ok := rep.find("gone", "rules"); ok {
		t.Errorf("a rules check for a removed account: %+v", c)
	}
}

func TestDoctorCleansTheFoldersOfRulesBeforeShowingThem(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	e.setRules("work", account.Rules{Folders: []string{"/tmp/x\x1b]0;owned\x07"}})

	_, rep := doctor(t, e)

	c, ok := rep.find("work", "rules")
	if !ok || hasControl(c.Detail) || !strings.Contains(c.Detail, `\x1b`) {
		t.Errorf("the rules check: %+v", c)
	}
}

func TestDoctorGoesOnWhenTheAccountListCannotBeRead(t *testing.T) {
	e := newEnv(t)
	plantState(t, "not json")

	code, rep := doctor(t, e)

	if _, ok := rep.find("state", "file"); !ok || code != 1 {
		t.Errorf("exit %d: %+v", code, rep.Checks)
	}
	for _, c := range rep.Checks {
		if c.Check == "rules" {
			t.Errorf("a rules check without an account list: %+v", c)
		}
	}
}
