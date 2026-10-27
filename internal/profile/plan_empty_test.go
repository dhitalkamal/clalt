package profile

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPlanReplacesAnEmptyFolderWithTheLinkButKeepsAFullOne(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	linkAll(t, acct, shared)
	for _, n := range []string{"commands", "agents"} {
		if err := os.Remove(filepath.Join(acct, n)); err != nil {
			t.Fatal(err)
		}
	}
	mkdirT(t, filepath.Join(acct, "commands"))
	writeT(t, filepath.Join(acct, "agents", "mine.md"), "own")
	// an empty file is not an empty folder
	if err := os.Remove(filepath.Join(acct, "settings.json")); err != nil {
		t.Fatal(err)
	}
	writeT(t, filepath.Join(acct, "settings.json"), "")

	got := stepMap(mustPlan(t, acct, shared))

	if len(got) != 3 {
		t.Fatalf("planned %v, want commands, agents and settings.json", got)
	}
	if st := got["commands"]; st.Action != Link || st.Source != filepath.Join(shared, "commands") || !strings.Contains(st.Why, "empty") {
		t.Errorf("commands = %+v, want a Link that says the empty folder is replaced", st)
	}
	for _, n := range []string{"agents", "settings.json"} {
		if got[n].Action != Keep {
			t.Errorf("%s = %+v, want Keep", n, got[n])
		}
	}
}

func TestSyncReplacesAnEmptyFolderAndLeavesAFullOne(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	mkdirT(t, filepath.Join(acct, "commands"))
	writeT(t, filepath.Join(acct, "agents", "mine.md"), "own")

	rep := mustSync(t, acct, shared)

	if target, err := os.Readlink(filepath.Join(acct, "commands")); err != nil || target != filepath.Join(shared, "commands") {
		t.Errorf("commands -> %q (%v), want a link to the shared folder", target, err)
	}
	if got := readT(t, filepath.Join(acct, "agents", "mine.md")); got != "own" {
		t.Errorf("the full folder was touched: %q", got)
	}
	if names := stepNames(rep.Kept); len(names) != 1 || names[0] != "agents" {
		t.Errorf("Kept = %v, want agents", names)
	}
	found := false
	for _, n := range rep.Linked {
		found = found || n == "commands"
	}
	if !found {
		t.Errorf("Linked = %v, want commands in it", rep.Linked)
	}
}

func TestApplyNeverRemovesAFolderThatFilledUpAfterPlanning(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	writeT(t, filepath.Join(acct, "commands", "new.md"), "just arrived")

	done, err := apply(acct, Step{Name: "commands", Action: Link, Source: filepath.Join(shared, "commands")})

	if err == nil || done {
		t.Errorf("apply = (%v, %v), want an error", done, err)
	}
	if got := readT(t, filepath.Join(acct, "commands", "new.md")); got != "just arrived" {
		t.Errorf("content lost: %q", got)
	}
}

func TestPlanKeepsAFolderItCannotList(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission bits are not enforced here")
	}
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	locked := filepath.Join(acct, "commands")
	mkdirT(t, locked)
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	st := stepMap(mustPlan(t, acct, shared))["commands"]

	if st.Action != Keep || !strings.Contains(st.Why, "cannot list") {
		t.Errorf("commands = %+v, want a Keep that says it cannot list the folder", st)
	}
}
