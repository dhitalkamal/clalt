package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/registry"
)

func TestRemoveDeleteForceDeletesFolderAndRecordButNotLinkTargets(t *testing.T) {
	e := newEnv(t)
	home := e.seedShared()
	e.signIn("work", "a@example.com")
	folder := e.folder("work")
	target := filepath.Join(home, "projects", "-p", "a.jsonl")

	code := e.run("remove", "work", "--delete", "--force")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if exists(folder) {
		t.Error("the account folder is still there")
	}
	if e.hasAccount("work") {
		t.Error("the record is still in state.json")
	}
	// the folder held links into ~/.claude, and only the links may go
	for _, p := range []string{target, filepath.Join(home, "settings.json"), filepath.Join(e.home, ".claude.json")} {
		if !exists(p) {
			t.Errorf("%s was deleted through a link", p)
		}
	}
	if len(e.claude.calls("logout")) == 0 {
		t.Error("the account was not signed out first")
	}
}

func TestRemoveDeleteSignOutFailureDeletesNothing(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	folder := e.folder("work")
	e.claude.set("LOGOUT_FAILS", "1")

	code := e.run("remove", "work", "--delete", "--force")

	if code != 1 || !strings.Contains(e.errOut.String(), "could not sign") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
	if !exists(folder) || !e.hasAccount("work") {
		t.Error("something was deleted after a failed sign-out")
	}
}

func TestRemoveDeleteRefusesASharedFolder(t *testing.T) {
	e := newEnv(t)
	plantState(t, collidingState)
	forceCollision(t, true)

	code := e.run("remove", "robin", "--delete", "--force")

	if code != 1 || !strings.Contains(e.errOut.String(), "share one folder") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
	if !e.hasAccount("robin") || !e.hasAccount("Robin") {
		t.Error("a record was deleted")
	}
	noClaudeCalls(t, e)
}

func TestRemoveDeleteDeclinedDeletesNothing(t *testing.T) {
	for _, reply := range []string{"n\n", "\n", ""} {
		e := newEnv(t)
		e.signIn("work", "a@example.com")
		folder := e.folder("work")
		e.answer(reply)

		e.run("remove", "work", "--delete")

		out := e.out.String()
		if !strings.Contains(out, "a@example.com") || !strings.Contains(out, folder) {
			t.Errorf("reply %q: the question should name the email and folder:\n%s", reply, out)
		}
		if !exists(folder) || !e.hasAccount("work") || len(e.claude.calls("logout")) != 0 {
			t.Errorf("reply %q: something changed", reply)
		}
	}
}

func TestRemoveDeleteConfirmedDeletes(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	folder := e.folder("work")
	e.answer("y\n")

	if code := e.run("remove", "work", "--delete"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if exists(folder) || e.hasAccount("work") {
		t.Error("a confirmed delete left something behind")
	}
}

func TestRemoveDeleteWorksOnAHiddenAccount(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	folder := e.folder("work")
	if code := e.run("remove", "work", "--force"); code != 0 {
		t.Fatal(e.errOut.String())
	}
	e.reset()

	if code := e.run("remove", "work", "--delete", "--force"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if exists(folder) || e.hasAccount("work") {
		t.Error("the hidden account was not deleted")
	}
}

func TestRemoveDeleteRefusesAFolderThatIsALink(t *testing.T) {
	skipWindows(t, "symlinks need privileges")
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	folder := e.folder("work")
	outside := t.TempDir()
	if err := os.RemoveAll(folder); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, folder); err != nil {
		t.Fatal(err)
	}

	code := e.run("remove", "work", "--delete", "--force")

	if code != 1 || !exists(outside) {
		t.Errorf("exit %d: a folder that is a link must be refused", code)
	}
	if !e.hasAccount("work") {
		t.Error("the record went")
	}
}

func TestPlainRemoveStillKeepsFolderAndRecord(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	folder := e.folder("work")

	if code := e.run("remove", "work", "--force"); code != 0 {
		t.Fatal(e.errOut.String())
	}
	if !exists(folder) || !e.hasAccount("work") || !e.account("work").Meta.Hidden {
		t.Error("plain remove must hide and keep")
	}
}

// the split-link layout puts real subfolders in the account folder, with links
// to shared folders inside them. none of that may be followed.
func TestRemoveDeleteDoesNotFollowLinksInsideSubfolders(t *testing.T) {
	skipWindows(t, "symlinks need privileges")
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	folder := e.folder("work")
	shared := filepath.Join(e.home, ".claude", "skills")
	if err := os.MkdirAll(filepath.Join(shared, "one"), 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(shared, "one", "SKILL.md")
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(folder, "plugins")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(shared, filepath.Join(sub, "skills")); err != nil {
		t.Fatal(err)
	}

	if code := e.run("remove", "work", "--delete", "--force"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if exists(folder) {
		t.Error("the folder is still there")
	}
	if !exists(keep) {
		t.Error("a file in the shared folder was deleted through a link")
	}
}

func TestRemoveDeleteRefusesALiveSessionEvenWithForce(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	folder := e.folder("work")
	reg := registry.Registry{Dir: config.SessionsDir()}
	rec := registry.Record{Handle: registry.NewHandle(), Account: e.account("work").Name,
		WrapperPID: os.Getpid(), PID: os.Getpid(), StartedAt: time.Now()}
	if err := reg.Create(rec); err != nil {
		t.Fatal(err)
	}

	code := e.run("remove", "work", "--delete", "--force")

	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, fmt.Sprint(os.Getpid())) || !strings.Contains(msg, "end") {
		t.Errorf("exit %d: %s", code, msg)
	}
	if !exists(folder) || !e.hasAccount("work") || len(e.claude.calls("logout")) != 0 {
		t.Error("something changed")
	}
}

// a removed account is not counted as sharing for other commands, but its
// folder is the same folder, so a delete must not take it away
func TestRemoveDeleteRefusesWhenAHiddenAccountSharesTheFolder(t *testing.T) {
	e := newEnv(t)
	plantState(t, strings.Replace(collidingState, `"Robin":{`, `"Robin":{"hidden":true,`, 1))
	forceCollision(t, true)

	code := e.run("remove", "robin", "--delete", "--force")

	if code != 1 || !strings.Contains(e.errOut.String(), `"Robin"`) {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
	if !e.hasAccount("robin") {
		t.Error("the record was deleted")
	}
	noClaudeCalls(t, e)
}
