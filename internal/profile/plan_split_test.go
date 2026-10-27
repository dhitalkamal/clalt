package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanLinksTheChildrenOfSkillsAndPluginsNotTheFolders(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	mkdirT(t, acct)

	steps := mustPlan(t, acct, shared)

	got := stepMap(steps)
	for _, n := range append(append([]string{}, skillsLinked...), pluginsLinked...) {
		st, ok := got[n]
		if !ok || st.Action != Link || st.Source != filepath.Join(shared, filepath.FromSlash(n)) || st.Folder {
			t.Errorf("%s = %+v, want a Link to the shared entry", n, st)
		}
	}
	// the account keeps its own copy of these, and the folders themselves are
	// real folders, never links
	for _, n := range []string{
		"skills", "plugins", "skills/synced", "skills/.trash",
		"plugins/synced", "plugins/.trash", "plugins/known_marketplaces_claudeai.json",
	} {
		if st, ok := got[n]; ok {
			t.Errorf("planned %+v for %s", st, n)
		}
	}
}

func TestPlanReplacesAWholeFolderLinkWithARealFolder(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	// the earlier layout: skills linked whole, and plugins linked to the user's own copy
	mustSymlink(t, filepath.Join(shared, "skills"), filepath.Join(acct, "skills"))
	own := filepath.Join(root, "my-plugins")
	mkdirT(t, own)
	mustSymlink(t, own, filepath.Join(acct, "plugins"))

	steps := mustPlan(t, acct, shared)

	got := stepMap(steps)
	for name, old := range map[string]string{"skills": filepath.Join(shared, "skills"), "plugins": own} {
		st := got[name]
		if st.Action != Repoint || !st.Folder || st.Old != old || st.Why == "" {
			t.Errorf("%s = %+v, want a Repoint that makes a real folder and remembers %s", name, st, old)
		}
	}
	// every child is planned as new, whatever the old link leads to
	for _, n := range append(append([]string{}, skillsLinked...), pluginsLinked...) {
		if got[n].Action != Link {
			t.Errorf("%s = %+v, want Link", n, got[n])
		}
	}
	// the folder is replaced before anything goes inside it
	order := stepNames(steps)
	if idx(order, "skills") > idx(order, "skills/review") || idx(order, "plugins") > idx(order, "plugins/cache") {
		t.Errorf("steps are ordered %v, want each folder before its children", order)
	}
}

func idx(names []string, name string) int {
	for i, n := range names {
		if n == name {
			return i
		}
	}
	return -1
}

func TestPlanAppliesTheSameRulesOneLevelDown(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	mkdirT(t, filepath.Join(shared, "skills", "empty"))
	acct := filepath.Join(root, "acct")
	linkAll(t, acct, shared)
	linkSplit(t, acct, shared)
	skills := filepath.Join(acct, "skills")

	// aimed elsewhere
	if err := os.Remove(filepath.Join(skills, "deploy")); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(root, "elsewhere")
	mkdirT(t, elsewhere)
	mustSymlink(t, elsewhere, filepath.Join(skills, "deploy"))
	// a real file where a link belongs
	if err := os.Remove(filepath.Join(skills, "README.md")); err != nil {
		t.Fatal(err)
	}
	writeT(t, filepath.Join(skills, "README.md"), "mine")
	// stale: source gone, and an excluded name linked by an earlier layout
	mustSymlink(t, filepath.Join(shared, "skills", "gone"), filepath.Join(skills, "gone"))
	mustSymlink(t, filepath.Join(shared, "skills", "synced"), filepath.Join(skills, "synced"))
	// an empty folder where a link belongs
	mkdirT(t, filepath.Join(skills, "empty"))
	// the account's own things, and a link of the user's
	mkdirT(t, filepath.Join(skills, "mine"))
	writeT(t, filepath.Join(skills, "mine", "x.md"), "own")
	mustSymlink(t, elsewhere, filepath.Join(skills, "theirs"))

	got := stepMap(mustPlan(t, acct, shared))

	want := map[string]Action{
		"skills/deploy": Repoint, "skills/README.md": Keep, "skills/gone": Remove,
		"skills/synced": Remove, "skills/empty": Link,
	}
	if len(got) != len(want) {
		t.Fatalf("planned %v, want exactly %v", stepNames(mapSteps(got)), want)
	}
	for name, act := range want {
		if got[name].Action != act {
			t.Errorf("%s = %+v, want %v", name, got[name], act)
		}
	}
	if got["skills/deploy"].Old != elsewhere || !strings.Contains(got["skills/gone"].Why, "gone") {
		t.Errorf("details wrong: %+v %+v", got["skills/deploy"], got["skills/gone"])
	}
}

func mapSteps(m map[string]Step) []Step {
	var out []Step
	for _, s := range m {
		out = append(out, s)
	}
	return out
}

func TestPlanKeepsAFileWhereASplitFolderBelongs(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	writeT(t, filepath.Join(acct, "skills"), "a file")

	got := stepMap(mustPlan(t, acct, shared))

	if st := got["skills"]; st.Action != Keep || !strings.Contains(st.Why, "file") {
		t.Errorf("skills = %+v, want a Keep about the file", st)
	}
	for name := range got {
		if strings.HasPrefix(name, "skills/") {
			t.Errorf("planned %s under a file", name)
		}
	}
}

func TestPlanLeavesSplitFoldersAloneWhenTheSharedHomeHasNone(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	linkAll(t, acct, shared)
	mkdirT(t, filepath.Join(acct, "skills", "mine"))
	mustSymlink(t, filepath.Join(shared, "plugins"), filepath.Join(acct, "plugins"))

	got := stepMap(mustPlan(t, acct, shared))

	// the dangling whole-folder link is the generic stale case
	if len(got) != 1 || got["plugins"].Action != Remove || !strings.Contains(got["plugins"].Why, "gone") {
		t.Errorf("planned %v, want only the dangling plugins link removed", got)
	}
}

func TestPlanIgnoresASplitNamedFileInTheSharedHome(t *testing.T) {
	root, shared := sharedFixture(t)
	writeT(t, filepath.Join(shared, "skills"), "not a folder")
	acct := filepath.Join(root, "acct")
	mkdirT(t, acct)

	for _, n := range stepNames(mustPlan(t, acct, shared)) {
		if strings.HasPrefix(n, "skills") {
			t.Errorf("planned %s for a file named skills", n)
		}
	}
}

func TestPlanKeepsAChildItCannotRead(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	reason := loopLink(t, filepath.Join(shared, "skills", "loop"))
	acct := filepath.Join(root, "acct")
	linkAll(t, acct, shared)
	linkSplit(t, acct, shared)
	mustSymlink(t, filepath.Join(shared, "skills", "loop"), filepath.Join(acct, "skills", "loop"))

	got := stepMap(mustPlan(t, acct, shared))

	if len(got) != 1 || got["skills/loop"].Action != Keep || !strings.Contains(got["skills/loop"].Why, reason) {
		t.Errorf("planned %v, want only a Keep for skills/loop carrying %q", got, reason)
	}
}

func TestPlanReadsASymlinkedSharedFolderButLinksThroughTheSharedHome(t *testing.T) {
	root, shared := sharedFixture(t)
	dotfiles := filepath.Join(root, "dotfiles", "skills")
	mkdirT(t, filepath.Join(dotfiles, "review"))
	mustSymlink(t, dotfiles, filepath.Join(shared, "skills"))
	acct := filepath.Join(root, "acct")
	mkdirT(t, acct)

	got := stepMap(mustPlan(t, acct, shared))

	// the link aims at the shared home entry, so it follows if that moves
	if st := got["skills/review"]; st.Action != Link || st.Source != filepath.Join(shared, "skills", "review") {
		t.Errorf("skills/review = %+v", st)
	}
}

func TestPlanIsEmptyWhenTheSplitFoldersAreInSync(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	linkAll(t, acct, shared)
	linkSplit(t, acct, shared)
	// what claude.ai puts there is the account's own
	writeT(t, filepath.Join(acct, "skills", "synced", "a", "SKILL.md"), "x")
	writeT(t, filepath.Join(acct, "plugins", "known_marketplaces_claudeai.json"), "{}")

	if steps := mustPlan(t, acct, shared); len(steps) != 0 {
		t.Errorf("planned %v", steps)
	}
}
