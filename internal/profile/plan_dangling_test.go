package profile

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// danglingShared replaces a shared entry with a link whose target is missing,
// which is how a link into an unmounted volume looks.
func danglingShared(t *testing.T, root, shared, name string) string {
	t.Helper()
	path := filepath.Join(shared, filepath.FromSlash(name))
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, filepath.Join(root, "volume", filepath.Base(name)), path)
	return path
}

func TestPlanKeepsTheLinkOfASharedEntryWhoseTargetIsMissing(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	linkAll(t, acct, shared)
	entry := danglingShared(t, root, shared, "commands")

	got := stepMap(mustPlan(t, acct, shared))

	st, ok := got["commands"]
	if len(got) != 1 || !ok || st.Action != Keep || st.Source != entry || !strings.Contains(st.Why, "missing") {
		t.Fatalf("planned %v, want one Keep for commands that says the target is missing", got)
	}
	rep := mustSync(t, acct, shared)
	if len(rep.Removed) != 0 || len(rep.Kept) != 1 || rep.Complete() {
		t.Errorf("report = %+v", rep)
	}
	if target, err := os.Readlink(filepath.Join(acct, "commands")); err != nil || target != entry {
		t.Errorf("the commands link is now %q (%v)", target, err)
	}
}

func TestPlanSaysNothingWhenTheLinkOfAMissingEntryIsNotClaltsOwn(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	linkAll(t, acct, shared)
	danglingShared(t, root, shared, "commands")
	// the user's own link under that name is not ours to report or to touch
	if err := os.Remove(filepath.Join(acct, "commands")); err != nil {
		t.Fatal(err)
	}
	mine := filepath.Join(root, "mine")
	mkdirT(t, mine)
	mustSymlink(t, mine, filepath.Join(acct, "commands"))

	if steps := mustPlan(t, acct, shared); len(steps) != 0 {
		t.Errorf("planned %v", steps)
	}
}

func TestPlanKeepsTheLinkOfADanglingEntryInsideASplitFolder(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	linkAll(t, acct, shared)
	linkSplit(t, acct, shared)
	entry := danglingShared(t, root, shared, "skills/review")

	got := stepMap(mustPlan(t, acct, shared))

	if st := got["skills/review"]; len(got) != 1 || st.Action != Keep || st.Source != entry {
		t.Fatalf("planned %v, want one Keep for skills/review", got)
	}
}

func TestAnUnmountedVolumeDoesNotCostTheAccountItsLinks(t *testing.T) {
	root, shared := sharedFixture(t)
	volume := filepath.Join(root, "volume")
	writeT(t, filepath.Join(volume, "projects", "chat.jsonl"), "history")
	if err := os.RemoveAll(filepath.Join(shared, "projects")); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, filepath.Join(volume, "projects"), filepath.Join(shared, "projects"))
	acct := filepath.Join(root, "acct")
	mustSync(t, acct, shared)
	unmounted := volume + ".off"
	if err := os.Rename(volume, unmounted); err != nil {
		t.Fatal(err)
	}

	rep := mustSync(t, acct, shared)

	if len(rep.Removed) != 0 || !reflect.DeepEqual(stepNames(rep.Kept), []string{"projects"}) {
		t.Errorf("while unmounted: report = %+v", rep)
	}
	if err := os.Rename(unmounted, volume); err != nil {
		t.Fatal(err)
	}
	if steps := mustPlan(t, acct, shared); len(steps) != 0 {
		t.Errorf("after mounting again: planned %v, want nothing to do", steps)
	}
	if got := readT(t, filepath.Join(acct, "projects", "chat.jsonl")); got != "history" {
		t.Errorf("history through the account link = %q", got)
	}
}
