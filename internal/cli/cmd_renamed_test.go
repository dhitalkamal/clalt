package cli

import (
	"strings"
	"testing"
)

// a name that is not an id: rename changes the label and never the id, and a
// fresh account can take the label that was freed. here "office" is the account
// that was signed in as work, with id work, and "work" is a new one with id
// work-2. a command that mixes up the two would act on the wrong folder.
func twoAccountsWhoseLabelsAreNotTheirIds(t *testing.T) *testEnv {
	t.Helper()
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	if code := e.run("rename", "work", "office"); code != 0 {
		t.Fatalf("rename exited %d: %s", code, e.errOut.String())
	}
	e.signIn("work", "b@example.com")
	if id := e.account("work").Name; id != "work-2" {
		t.Fatalf("setup: the new account has id %q, want work-2", id)
	}
	return e
}

func TestAFreshAccountOnAFreedLabelHasItsOwnIdentityAndFolder(t *testing.T) {
	e := twoAccountsWhoseLabelsAreNotTheirIds(t)

	old, fresh := e.account("office"), e.account("work")

	if old.Name != "work" || old.Meta.Email != "a@example.com" {
		t.Errorf("the renamed account = %+v", old)
	}
	if fresh.Name != "work-2" || fresh.Meta.Email != "b@example.com" {
		t.Errorf("the new account = %+v", fresh)
	}
	if e.folder("office") == e.folder("work") || !e.claude.signedIn(e.folder("office")) || !e.claude.signedIn(e.folder("work")) {
		t.Error("each account has its own folder and its own login")
	}
}

func TestRunStartsClaudeInTheFolderOfTheAccountThatTheLabelNames(t *testing.T) {
	for label, id := range map[string]string{"office": "work", "work": "work-2"} {
		t.Run(label, func(t *testing.T) {
			e := twoAccountsWhoseLabelsAreNotTheirIds(t)

			if code := e.run("run", label); code != 0 {
				t.Fatalf("exit %d: %s", code, e.errOut.String())
			}

			_, env, _ := e.claude.session()
			if want := accountDir(t, id); env["CLAUDE_CONFIG_DIR"] != want {
				t.Errorf("the session ran in %q, want %q", env["CLAUDE_CONFIG_DIR"], want)
			}
			if e.account(label).Meta.LastUsedAt.IsZero() {
				t.Errorf("%s was not marked as used", label)
			}
			other := "work"
			if label == "work" {
				other = "office"
			}
			if !e.account(other).Meta.LastUsedAt.IsZero() {
				t.Errorf("%s was marked as used by a run of %s", other, label)
			}
		})
	}
}

func TestLogoutSignsOutOnlyTheFolderOfTheLabel(t *testing.T) {
	e := twoAccountsWhoseLabelsAreNotTheirIds(t)
	office, fresh := e.folder("office"), e.folder("work")

	if code := e.run("logout", "office"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if e.claude.signedIn(office) || !e.claude.signedIn(fresh) {
		t.Errorf("office signed in %v, work signed in %v: only office should be signed out", e.claude.signedIn(office), e.claude.signedIn(fresh))
	}
	if m := e.account("office").Meta; m.Email != "a@example.com" || !m.SignedInAt.IsZero() {
		t.Errorf("office meta = %+v", m)
	}
	if m := e.account("work").Meta; m.Email != "b@example.com" || m.SignedInAt.IsZero() {
		t.Errorf("work meta = %+v: another account was touched", m)
	}
	if out := e.out.String(); !strings.Contains(out, "office is signed out") || !strings.Contains(out, "clalt login office") {
		t.Errorf("the output names accounts by label:\n%s", out)
	}
}

func TestRemoveHidesOnlyTheAccountOfTheLabelAndKeepsItsFolder(t *testing.T) {
	e := twoAccountsWhoseLabelsAreNotTheirIds(t)
	office, fresh := e.folder("office"), e.folder("work")

	if code := e.run("remove", "office", "--force"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if !e.account("office").Meta.Hidden || e.account("work").Meta.Hidden {
		t.Error("only office should be removed")
	}
	if e.claude.signedIn(office) || !e.claude.signedIn(fresh) {
		t.Error("only the folder of office should have been signed out")
	}
	if !exists(office) || !exists(fresh) {
		t.Error("a folder was deleted")
	}
	if out := e.out.String(); !strings.Contains(out, "removed office") || !strings.Contains(out, "clalt login office") {
		t.Errorf("the output names accounts by label:\n%s", out)
	}
	if got := labelsInList(t, e); len(got) != 1 || got[0] != "work" {
		t.Errorf("list = %v, want only work", got)
	}
}

func TestDoctorAndListNameEachAccountByItsLabelAndLookAtItsOwnFolder(t *testing.T) {
	e := twoAccountsWhoseLabelsAreNotTheirIds(t)
	e.claude.signOut(e.folder("office"))

	_, rep := doctor(t, e)

	rep.want(t, "office", "signedIn", "warn", "clalt login office")
	rep.want(t, "work", "signedIn", "ok", "")
	rep.want(t, "work", "identity", "ok", "b@example.com")
	rows := listJSON(t, e)
	byLabel := map[string]map[string]any{}
	for _, r := range rows {
		byLabel[r["label"].(string)] = r
	}
	if byLabel["office"]["id"] != "work" || byLabel["office"]["signedIn"] != false {
		t.Errorf("office row = %v", byLabel["office"])
	}
	if byLabel["work"]["id"] != "work-2" || byLabel["work"]["signedIn"] != true {
		t.Errorf("work row = %v", byLabel["work"])
	}
}

// what login prints names accounts by label, never by id
func TestLoginNamesARenamedAccountByLabel(t *testing.T) {
	t.Run("the email belongs to it", func(t *testing.T) {
		e := twoAccountsWhoseLabelsAreNotTheirIds(t)
		e.claude.signInAs("a@example.com")

		e.run("login", "newcomer")

		if msg := e.errOut.String(); !strings.Contains(msg, `already belongs to "office"`) {
			t.Errorf("stderr: %s", msg)
		}
	})
	t.Run("another email signed in", func(t *testing.T) {
		e := twoAccountsWhoseLabelsAreNotTheirIds(t)
		e.claude.signInAs("c@example.com")

		e.run("login", "office")

		if msg := e.errOut.String(); !strings.Contains(msg, `"office" is set up for a@example.com`) {
			t.Errorf("stderr: %s", msg)
		}
	})
	t.Run("the forced sign-out fails", func(t *testing.T) {
		e := twoAccountsWhoseLabelsAreNotTheirIds(t)
		e.claude.signInAs("c@example.com")
		e.claude.set("LOGOUT_FAILS", "1")

		e.run("login", "office")

		if msg := e.errOut.String(); !strings.Contains(msg, "'clalt logout office'") {
			t.Errorf("stderr: %s", msg)
		}
	})
	t.Run("the sign-in worked", func(t *testing.T) {
		e := twoAccountsWhoseLabelsAreNotTheirIds(t)
		e.claude.signInAs("a@example.com")

		e.run("login", "office")

		if out := e.out.String(); !strings.Contains(out, "office is signed in as a@example.com") {
			t.Errorf("stdout: %s", out)
		}
	})
	t.Run("the history is not reachable", func(t *testing.T) {
		e := twoAccountsWhoseLabelsAreNotTheirIds(t)
		unmountHistory(t, e)
		e.claude.signInAs("a@example.com")

		e.run("login", "office")

		if msg := e.errOut.String(); !strings.Contains(msg, "'clalt run office' will refuse") {
			t.Errorf("stderr: %s", msg)
		}
	})
}

// the identity of a sign-in goes to the account that signed in, whatever its
// label is, and a failed first login hides the account it made
func TestAFailedFirstLoginHidesTheNewAccountAndNotTheRenamedOne(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	if code := e.run("rename", "work", "office"); code != 0 {
		t.Fatalf("rename exited %d: %s", code, e.errOut.String())
	}
	e.claude.set("LOGIN_FAILS", "1")

	if code := e.run("login", "work"); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, e.errOut.String())
	}

	if e.account("office").Meta.Hidden {
		t.Error("the failed login hid another account")
	}
	if fresh := e.account("work"); !fresh.Meta.Hidden || fresh.Name != "work-2" {
		t.Errorf("the account that was made: %+v", fresh)
	}
}

func TestARemovedAndRenamedAccountComesBackByItsLabel(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	if code := e.run("remove", "work", "--force"); code != 0 {
		t.Fatalf("remove exited %d: %s", code, e.errOut.String())
	}
	if code := e.run("rename", "work", "old-work"); code != 0 {
		t.Fatalf("rename exited %d: %s", code, e.errOut.String())
	}
	e.reset()
	e.claude.signInAs("c@example.com")

	if code := e.run("login", "old-work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if m := e.account("old-work").Meta; m.Hidden || m.Email != "c@example.com" {
		t.Errorf("meta = %+v", m)
	}
	if e.account("old-work").Name != "work" {
		t.Error("it came back in another folder")
	}
}
