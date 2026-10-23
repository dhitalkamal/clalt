package profile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanAndSyncRefuseEmptyAndRelativePaths(t *testing.T) {
	root, shared := sharedFixture(t)
	t.Chdir(root)
	acct := filepath.Join(root, "acct")
	cases := []struct {
		name            string
		profile, shared string
	}{
		{"empty shared home", acct, ""},
		{"relative shared home", acct, "shared"},
		{"empty profile folder", "", shared},
		{"relative profile folder", "acct", shared},
	}
	for _, c := range cases {
		if steps, err := Plan(c.profile, c.shared); err == nil {
			t.Errorf("%s: Plan planned %v", c.name, steps)
		}
		if _, err := Sync(c.profile, c.shared); err == nil {
			t.Errorf("%s: Sync accepted it", c.name)
		}
	}
	// nothing was created for any of them, not even the folder
	for _, p := range []string{"acct", acct} {
		if present(p) {
			t.Errorf("%s was created", p)
		}
	}
}

func TestMCPSyncRefusesEmptyAndRelativePaths(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	sharedJSON := filepath.Join(root, "shared.claude.json")
	putJSON(t, sharedJSON, serversJSON("a", defA))
	acct := filepath.Join(root, "acct")
	cases := []struct {
		name            string
		profile, shared string
	}{
		{"empty shared config", acct, ""},
		{"relative shared config", acct, "shared.claude.json"},
		{"empty account folder", "", sharedJSON},
		{"relative account folder", "acct", sharedJSON},
	}
	for _, c := range cases {
		if _, err := SyncMCP(c.profile, c.shared); err == nil {
			t.Errorf("%s: SyncMCP accepted it", c.name)
		}
		if _, err := PlanMCP(c.profile, c.shared); err == nil {
			t.Errorf("%s: PlanMCP accepted it", c.name)
		}
	}
	if present("acct") || present(acct) {
		t.Error("an account folder was created for rejected input")
	}
}

func TestAMissingSharedHomeIsAnErrorThatRemovesNothing(t *testing.T) {
	root := t.TempDir()
	shared := filepath.Join(root, "no-such-home")
	acct := filepath.Join(root, "acct")
	// links into the missing home look dangling, but the home may only be unmounted
	mustSymlink(t, filepath.Join(shared, "commands"), filepath.Join(acct, "commands"))
	mustSymlink(t, filepath.Join(shared, "agents"), filepath.Join(acct, "agents"))

	steps, err := Plan(acct, shared)
	if !errors.Is(err, ErrNoSharedHome) || steps != nil {
		t.Fatalf("Plan = %v, %v, want ErrNoSharedHome", steps, err)
	}
	if got := err.Error(); !strings.Contains(got, shared) {
		t.Errorf("error %q does not name the missing path", got)
	}

	rep, err := Sync(acct, shared)
	if !errors.Is(err, ErrNoSharedHome) {
		t.Fatalf("Sync err = %v, want ErrNoSharedHome", err)
	}
	if len(rep.Removed)+len(rep.Linked) != 0 {
		t.Errorf("report = %+v", rep)
	}
	for _, n := range []string{"commands", "agents"} {
		if _, err := os.Readlink(filepath.Join(acct, n)); err != nil {
			t.Errorf("%s was removed: %v", n, err)
		}
	}
}

func TestSyncCreatesTheFolderEvenWithoutASharedHome(t *testing.T) {
	root := t.TempDir()
	acct := filepath.Join(root, "accounts", "robin")

	_, err := Sync(acct, filepath.Join(root, "no-such-home"))

	if !errors.Is(err, ErrNoSharedHome) {
		t.Fatalf("err = %v", err)
	}
	if fi, statErr := os.Stat(acct); statErr != nil || !fi.IsDir() {
		t.Errorf("the account folder should exist for the login that follows: %v", statErr)
	}
}
