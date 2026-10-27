package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// adoptEnv has an account whose plugins/known_marketplaces.json was replaced by
// a real, newer file, as claude does when it saves.
func adoptEnv(t *testing.T) (e *testEnv, shared, accFile string) {
	t.Helper()
	e = newEnv(t)
	home := e.seedShared()
	shared = filepath.Join(home, "plugins", "known_marketplaces.json")
	if err := os.MkdirAll(filepath.Dir(shared), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shared, []byte("old shared"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(shared, old, old); err != nil {
		t.Fatal(err)
	}
	e.signIn("work", "w@example.com")
	accFile = filepath.Join(e.folder("work"), "plugins", "known_marketplaces.json")
	if err := os.Remove(accFile); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(accFile, []byte("new account"), 0o600); err != nil {
		t.Fatal(err)
	}
	return e, shared, accFile
}

func TestDoctorShowsAPendingAdoptWithoutChangingAnything(t *testing.T) {
	e, _, _ := adoptEnv(t)
	before := snapshot(t, e.home)

	code, rep := doctor(t, e)

	rep.want(t, "work", "links", "warn", "plugins/known_marketplaces.json")
	rep.want(t, "work", "links", "warn", "'clalt run work' will adopt")
	if c, _ := rep.find("work", "links"); strings.Contains(c.Detail, "in the way") {
		t.Errorf("an adopt is not in the way: %s", c.Detail)
	}
	if code != 0 {
		t.Errorf("exit %d", code)
	}
	if after := snapshot(t, e.home); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Error("doctor changed something")
	}
}

func TestRunAdoptsTheNewerCopyAndSaysSo(t *testing.T) {
	e, shared, accFile := adoptEnv(t)

	e.reset()
	code := e.run("run", "work")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	out := e.errOut.String()
	if !strings.Contains(out, "adopted plugins/known_marketplaces.json (newer copy kept, older backed up to ") {
		t.Errorf("no adopt line:\n%s", out)
	}
	if strings.Contains(out, "left as it is") {
		t.Errorf("a left-as-is note remains:\n%s", out)
	}
	if b, _ := os.ReadFile(shared); string(b) != "new account" {
		t.Errorf("shared = %q", b)
	}
	if fi, err := os.Lstat(accFile); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("account entry is not a link: %v", err)
	}
	backups, _ := filepath.Glob(filepath.Join(e.home, ".config", "clalt", "backups", "plugins", e.account("work").Name, "known_marketplaces.json.*"))
	if len(backups) != 1 {
		t.Fatalf("backups = %v", backups)
	}
	if b, _ := os.ReadFile(backups[0]); string(b) != "old shared" {
		t.Errorf("backup = %q", b)
	}

	e.reset()
	e.run("run", "work")
	if strings.Contains(e.errOut.String(), "adopted") {
		t.Errorf("second run adopted again:\n%s", e.errOut.String())
	}
}
