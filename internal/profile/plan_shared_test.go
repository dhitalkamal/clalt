package profile

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestPlanClassifiesSharedSymlinksByTheirTarget(t *testing.T) {
	root, shared := sharedFixture(t)
	elsewhere := filepath.Join(root, "dotfiles")
	mkdirT(t, filepath.Join(elsewhere, "hooks"))
	writeT(t, filepath.Join(elsewhere, "keybindings.json"), "{}")
	writeT(t, filepath.Join(elsewhere, "notes.txt"), "x")
	mustSymlink(t, filepath.Join(elsewhere, "hooks"), filepath.Join(shared, "hooks"))
	mustSymlink(t, filepath.Join(elsewhere, "keybindings.json"), filepath.Join(shared, "keybindings.json"))
	mustSymlink(t, filepath.Join(elsewhere, "notes.txt"), filepath.Join(shared, "notes.txt"))
	// a symlink to a folder named like a per-account one stays out
	mustSymlink(t, filepath.Join(elsewhere, "hooks"), filepath.Join(shared, "telemetry"))
	profileDir := filepath.Join(root, "acct")

	got := stepMap(mustPlan(t, profileDir, shared))

	for _, n := range []string{"hooks", "keybindings.json"} {
		// the link aims at the shared home entry, not at what that entry points to
		if got[n].Action != Link || got[n].Source != filepath.Join(shared, n) {
			t.Errorf("%s: %+v, want Link to %s", n, got[n], filepath.Join(shared, n))
		}
	}
	for _, n := range []string{"notes.txt", "telemetry"} {
		if _, ok := got[n]; ok {
			t.Errorf("%s should not be linked", n)
		}
	}
}

// a fresh account has no link to protect, so a shared entry whose target is
// missing is not worth a word.
func TestPlanSaysNothingAboutADanglingEntryThatWasNeverLinked(t *testing.T) {
	root, shared := sharedFixture(t)
	mustSymlink(t, filepath.Join(root, "gone"), filepath.Join(shared, "hooks"))
	mustSymlink(t, filepath.Join(root, "gone.md"), filepath.Join(shared, "keybindings.json"))
	profileDir := filepath.Join(root, "acct")

	got := stepMap(mustPlan(t, profileDir, shared))

	for _, n := range []string{"hooks", "keybindings.json"} {
		if _, ok := got[n]; ok {
			t.Errorf("dangling shared entry %s was planned: %+v", n, got[n])
		}
	}
}

func TestPlanRefusesAnEmptySharedHome(t *testing.T) {
	root, _ := sharedFixture(t)
	t.Chdir(root)
	// an empty path must not quietly mean the working directory
	if steps, err := Plan(filepath.Join(root, "acct"), ""); err == nil {
		t.Errorf("Plan accepted an empty shared home and planned %v", steps)
	}
}

func TestPlanRefusesTheSharedHomeAsProfileFolder(t *testing.T) {
	_, shared := sharedFixture(t)
	if _, err := Plan(shared, shared); err == nil {
		t.Error("Plan accepted the shared home as the profile folder")
	}
}

func TestPlanLeavesTheTreeUntouched(t *testing.T) {
	root, shared := sharedFixture(t)
	profileDir := filepath.Join(root, "acct")
	linkAll(t, profileDir, shared)
	mustSymlink(t, filepath.Join(shared, "sessions"), filepath.Join(profileDir, "sessions"))
	writeT(t, filepath.Join(profileDir, "stray.txt"), "x")
	before := listTree(t, root)

	mustPlan(t, profileDir, shared)

	if after := listTree(t, root); !reflect.DeepEqual(before, after) {
		t.Errorf("Plan changed the tree:\nbefore %v\nafter  %v", before, after)
	}
}
