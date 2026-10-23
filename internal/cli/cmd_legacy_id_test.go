package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// v0.1.0 let names through that account.ValidateName turns down today, and an
// id never changes, so those accounts have to keep working. all an id has to be
// is one safe folder name, which is what config.AccountDirFor checks.
func TestALegacyIdThatIsASafeFolderNameKeepsWorking(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	plantState(t, `{"active":"","accounts":{"weird name!":{"email":"w@example.com","addedAt":"2026-01-01T00:00:00Z"}}}`)
	e.claude.signInAs("w@example.com")
	folder := filepath.Join(config.AccountsDir(), "weird name!")

	if code := e.run("login", "weird name!"); code != 0 {
		t.Fatalf("login: exit %d: %s", code, e.errOut.String())
	}
	e.reset()
	if code := e.run("run", "weird name!"); code != 0 {
		t.Fatalf("run: exit %d: %s", code, e.errOut.String())
	}
	if !exists(folder) {
		t.Errorf("the folder %s is not named after the id", folder)
	}
	e.reset()
	if row := listJSON(t, e)[0]; row["signedIn"] != true || row["folder"] != folder {
		t.Errorf("row = %v, want signed in, in %s", row, folder)
	}
	e.reset()
	if code := e.run("logout", "weird name!"); code != 0 {
		t.Errorf("logout: exit %d: %s", code, e.errOut.String())
	}
	e.reset()
	if code := e.run("remove", "weird name!", "--force"); code != 0 {
		t.Errorf("remove: exit %d: %s", code, e.errOut.String())
	}
}

// the stricter check is for names people type: a new label, or a rename.
func TestANewLabelStillHasToPassTheStricterCheck(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	if code := e.run("login", "new name!"); code != 1 || e.hasAccount("new name!") {
		t.Errorf("login: exit %d, account created %v: %s", code, e.hasAccount("new name!"), e.errOut.String())
	}
	e.reset()
	if code := e.run("rename", "work", "bad name!"); code != 1 || !strings.Contains(e.errOut.String(), "name") {
		t.Errorf("rename: exit %d: %s", code, e.errOut.String())
	}
	if exists(filepath.Join(config.AccountsDir(), "new name!")) {
		t.Error("a folder appeared for a label that was refused")
	}
}
