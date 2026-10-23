package profile

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSyncCleansTheLinksOfASplitFolderThatLeftTheSharedHome(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	mustSync(t, acct, shared)
	// what the account owns itself must survive the cleanup
	writeT(t, filepath.Join(acct, "skills", "synced", "a", "SKILL.md"), "own")
	writeT(t, filepath.Join(acct, "skills", "mine", "SKILL.md"), "mine")
	elsewhere := filepath.Join(root, "elsewhere")
	mkdirT(t, elsewhere)
	mustSymlink(t, elsewhere, filepath.Join(acct, "skills", "linked"))
	if err := os.RemoveAll(filepath.Join(shared, "skills")); err != nil {
		t.Fatal(err)
	}

	rep := mustSync(t, acct, shared)

	if !reflect.DeepEqual(rep.Removed, skillsLinked) {
		t.Errorf("Removed = %v, want %v", rep.Removed, skillsLinked)
	}
	for _, f := range []string{"synced/a/SKILL.md", "mine/SKILL.md", "linked"} {
		if !present(filepath.Join(acct, "skills", filepath.FromSlash(f))) {
			t.Errorf("skills/%s is gone", f)
		}
	}
	for _, n := range pluginsLinked {
		if _, err := os.Readlink(filepath.Join(acct, filepath.FromSlash(n))); err != nil {
			t.Errorf("%s lost its link: %v", n, err)
		}
	}
	if steps := mustPlan(t, acct, shared); len(steps) != 0 {
		t.Errorf("planned %v after the cleanup", steps)
	}
}

func TestPlanKeepsEverythingWhenASplitFolderIsALinkToNothing(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	mustSync(t, acct, shared)
	entry := danglingShared(t, root, shared, "skills")

	got := stepMap(mustPlan(t, acct, shared))

	st := got["skills"]
	if len(got) != 1 || st.Action != Keep || st.Source != entry || !strings.Contains(st.Why, "missing") {
		t.Fatalf("planned %v, want one Keep for skills", got)
	}
	if rep := mustSync(t, acct, shared); len(rep.Removed) != 0 {
		t.Errorf("Removed = %v while the shared folder is unreachable", rep.Removed)
	}
	for _, n := range skillsLinked {
		if _, err := os.Readlink(filepath.Join(acct, filepath.FromSlash(n))); err != nil {
			t.Errorf("%s lost its link: %v", n, err)
		}
	}

	// an account that made no links into it has nothing to report
	bare := filepath.Join(root, "bare")
	writeT(t, filepath.Join(bare, "skills", "mine", "SKILL.md"), "mine")
	if steps := stepMap(mustPlan(t, bare, shared)); steps["skills"].Action != 0 {
		t.Errorf("an account without links got %+v", steps["skills"])
	}
}

func TestPlanRemovesAWholeFolderLinkWhoseSharedFolderIsGone(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	mustSymlink(t, filepath.Join(shared, "skills"), filepath.Join(acct, "skills"))
	if err := os.RemoveAll(filepath.Join(shared, "skills")); err != nil {
		t.Fatal(err)
	}

	steps := mustPlan(t, acct, shared)

	got := stepMap(steps)
	for _, st := range steps {
		if strings.HasPrefix(st.Name, "skills") && st.Name != "skills" {
			t.Errorf("planned %+v inside a link that is going away", st)
		}
	}
	if st := got["skills"]; st.Action != Remove || !strings.Contains(st.Why, "gone") {
		t.Errorf("skills = %+v, want the old link removed because its source is gone", st)
	}
}

func TestPlanLeavesASplitFolderAloneWhenTheSharedNameBecameAFile(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	mustSync(t, acct, shared)
	if err := os.RemoveAll(filepath.Join(shared, "skills")); err != nil {
		t.Fatal(err)
	}
	writeT(t, filepath.Join(shared, "skills"), "not a folder")

	for _, st := range mustPlan(t, acct, shared) {
		if strings.HasPrefix(st.Name, "skills") {
			t.Errorf("planned %+v for skills while the shared name is a file", st)
		}
	}
}

func TestPlanDoesNotLookInsideALinkedFolderForStaleLinks(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	own := filepath.Join(root, "own-skills")
	// the user's own folder, linked in as skills, with a link that looks like ours
	mustSymlink(t, filepath.Join(shared, "skills", "review"), filepath.Join(own, "review"))
	mustSymlink(t, own, filepath.Join(acct, "skills"))

	for _, st := range mustPlan(t, acct, shared) {
		if strings.HasPrefix(st.Name, "skills/") {
			t.Errorf("planned %+v inside a linked folder", st)
		}
	}
}

func TestPlanKeepsEverythingWhenASplitFolderCannotBeExamined(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	mustSync(t, acct, shared)
	if err := os.RemoveAll(filepath.Join(shared, "skills")); err != nil {
		t.Fatal(err)
	}
	loopLink(t, filepath.Join(shared, "skills"))

	got := stepMap(mustPlan(t, acct, shared))

	if st := got["skills"]; len(got) != 1 || st.Action != Keep {
		t.Errorf("planned %v, want one Keep for skills and no removals", got)
	}
}
