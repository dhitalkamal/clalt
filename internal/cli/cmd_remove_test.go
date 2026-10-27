package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveSignsOutThenHidesAndKeepsTheHistory(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	folder := e.folder("work")

	code := e.run("remove", "work", "--force")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if e.claude.signedIn(folder) {
		t.Error("a login was left behind in the hidden folder")
	}
	m := e.account("work").Meta
	if !m.Hidden || m.Email != "" || m.Org != "" || m.Plan != "" || !m.SignedInAt.IsZero() {
		t.Errorf("meta = %+v: hidden, and the identity forgotten", m)
	}
	if !exists(folder) || !exists(filepath.Join(folder, "projects")) {
		t.Error("the folder and its history link must stay")
	}
	out := e.out.String()
	if !strings.Contains(out, "history stays") || !strings.Contains(out, "clalt login work") {
		t.Errorf("the output should say history stays and how to get back:\n%s", out)
	}
}

func TestRemoveAsksFirstAndNoChangesNothing(t *testing.T) {
	for _, reply := range []string{"n\n", "no\n", "\n", "yes please\n", "N", "nope"} {
		e := newEnv(t)
		e.signIn("work", "a@example.com")
		e.answer(reply)

		code := e.run("remove", "work")

		if code != 0 || !strings.Contains(e.out.String(), "aborted") {
			t.Errorf("reply %q: exit %d, output %q", reply, code, e.out.String())
		}
		if e.account("work").Meta.Hidden || len(e.claude.calls("logout")) != 0 {
			t.Errorf("reply %q: something changed", reply)
		}
	}
}

// no answer at all is not a "no": the input ended before anybody was asked, as it
// does in a script, and a script has to be told that nothing was removed
func TestRemoveWithoutAnAnswerIsAnErrorThatPointsAtForce(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.answer("")

	code := e.run("remove", "work")

	if code != 1 || !strings.Contains(e.errOut.String(), "--force") || !strings.Contains(e.errOut.String(), "not removed") {
		t.Errorf("exit %d, stderr %q: want 1, saying nothing was removed and how to do it without asking", code, e.errOut.String())
	}
	if strings.Contains(e.out.String(), "aborted") {
		t.Errorf("stdout %q: nobody said no", e.out.String())
	}
	if e.account("work").Meta.Hidden || len(e.claude.calls("logout")) != 0 {
		t.Error("something changed")
	}
}

// an answer that is cut off by the end of the input is still an answer
func TestRemoveTakesAYesThatEndsTheInput(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.answer("y")

	if code := e.run("remove", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if !e.account("work").Meta.Hidden {
		t.Error("the account was not removed")
	}
}

func TestRemoveGoesAheadOnYes(t *testing.T) {
	for _, reply := range []string{"y\n", "Y\n", "yes\n"} {
		e := newEnv(t)
		e.signIn("work", "a@example.com")
		e.answer(reply)

		if code := e.run("rm", "work"); code != 0 {
			t.Fatalf("reply %q: exit %d: %s", reply, code, e.errOut.String())
		}

		if !e.account("work").Meta.Hidden {
			t.Errorf("reply %q: the account was not removed", reply)
		}
	}
}

func TestRemoveThatCannotSignOutStopsWithoutHiding(t *testing.T) {
	e := newEnv(t)
	e.at(theClock)
	e.signIn("work", "a@example.com")
	before := e.account("work").Meta
	e.claude.set("LOGOUT_FAILS", "1")

	code := e.run("remove", "work", "--force")

	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, "could not reach the keychain") {
		t.Errorf("exit %d: %s", code, msg)
	}
	if after := e.account("work").Meta; after != before {
		t.Errorf("meta changed: %+v then %+v", before, after)
	}
}

func TestRemoveOfAnAccountThatIsSignedOutSkipsTheLogout(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.run("logout", "work")
	before := len(e.claude.calls("logout"))

	if code := e.run("remove", "work", "--force"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := len(e.claude.calls("logout")); got != before {
		t.Errorf("logout was called again (%d then %d)", before, got)
	}
	if !e.account("work").Meta.Hidden {
		t.Error("the account was not removed")
	}
}

func TestRemoveThatCannotTellIfItIsSignedInStopsWithoutHiding(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.claude.set("STATUS_BROKEN", "1")

	code := e.run("remove", "work", "--force")

	if code != 1 || !strings.Contains(e.errOut.String(), "keychain is locked") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
	if e.account("work").Meta.Hidden {
		t.Error("the account was hidden while a login may remain")
	}
}

func TestRemoveOfSomeoneUnknownOrAlreadyRemovedIsAnError(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.run("remove", "work", "--force")
	for _, label := range []string{"nobody", "work"} {
		e.reset()
		if code := e.run("remove", label, "--force"); code != 1 || e.errOut.Len() == 0 {
			t.Errorf("%s: exit %d, stderr %q", label, code, e.errOut.String())
		}
	}
}

func TestRemoveMistakesAreUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"remove"}, {"remove", "a", "b"}, {"remove", "a", "--nope"}} {
		e := newEnv(t)
		code := e.run(args...)
		if code != 2 || !strings.Contains(e.errOut.String(), "usage: clalt remove <name> [--force]") {
			t.Errorf("%v: exit %d, stderr %q", args, code, e.errOut.String())
		}
	}
}

func TestRenameChangesTheLabelAndNeverMovesTheFolder(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	folder := e.folder("work")
	if err := os.WriteFile(filepath.Join(folder, "marker"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	code := e.run("rename", "work", "office")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	acc := e.account("office")
	if acc.Name != "work" || e.folder("office") != folder || !exists(filepath.Join(folder, "marker")) {
		t.Errorf("account %+v, folder %s: only the label should change", acc, e.folder("office"))
	}
	if e.hasAccount("work") {
		t.Error("the old label still finds the account")
	}
	if got := e.account("office").Meta; got.Email != "a@example.com" || got.SignedInAt.IsZero() {
		t.Errorf("the sign-in was touched: %+v", got)
	}
	// and it still runs, in the same folder, under the new label
	if code := e.run("run", "office"); code != 0 {
		t.Errorf("run office: exit %d: %s", code, e.errOut.String())
	}
}

func TestRenameFreesTheOldLabelForAFreshAccount(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.run("remove", "work", "--force")
	if code := e.run("rename", "work", "old-work"); code != 0 {
		t.Fatalf("a removed account can be renamed: exit %d: %s", code, e.errOut.String())
	}

	e.signIn("work", "b@example.com")

	if e.account("work").Name == e.account("old-work").Name {
		t.Error("the fresh account reused the old one")
	}
}

func TestRenameRefusals(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"a label that is taken", []string{"rename", "work", "HOME"}, "already exists"},
		{"a name that cannot be a label", []string{"rename", "work", "../x"}, "only letters"},
		{"nobody", []string{"rename", "nobody", "x"}, "no account called"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.signIn("work", "a@example.com")
			e.signIn("home", "b@example.com")

			code := e.run(c.args...)

			if code != 1 || !strings.Contains(e.errOut.String(), c.want) {
				t.Errorf("exit %d: %s, want 1 and %q", code, e.errOut.String(), c.want)
			}
			if !e.hasAccount("work") {
				t.Error("the account lost its label")
			}
		})
	}
}

func TestRenameMistakesAreUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"rename"}, {"rename", "a"}, {"rename", "a", "b", "c"}, {"rename", "--nope", "b"}} {
		e := newEnv(t)
		code := e.run(args...)
		if code != 2 || !strings.Contains(e.errOut.String(), "usage: clalt rename <old> <new>") {
			t.Errorf("%v: exit %d, stderr %q", args, code, e.errOut.String())
		}
	}
}
