package profile

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPlanLinksEveryShareableEntryIntoEmptyProfile(t *testing.T) {
	root, shared := sharedFixture(t)
	profileDir := filepath.Join(root, "acct")
	mkdirT(t, profileDir)

	steps := mustPlan(t, profileDir, shared)

	var names []string
	for _, s := range steps {
		names = append(names, s.Name)
		if s.Action != Link {
			t.Errorf("%s: action %v, want Link", s.Name, s.Action)
		}
		if want := filepath.Join(shared, s.Name); s.Source != want {
			t.Errorf("%s: source %q, want %q", s.Name, s.Source, want)
		}
		if s.Why == "" {
			t.Errorf("%s: empty Why", s.Name)
		}
	}
	if !reflect.DeepEqual(names, shareableNames) {
		t.Errorf("planned %v, want %v (sorted by name)", names, shareableNames)
	}
}

func TestPlanDoesNotCreateTheProfileFolder(t *testing.T) {
	root, shared := sharedFixture(t)
	profileDir := filepath.Join(root, "not-yet")

	steps := mustPlan(t, profileDir, shared)

	if len(steps) != len(shareableNames) {
		t.Errorf("planned %d steps, want %d", len(steps), len(shareableNames))
	}
	if present(profileDir) {
		t.Error("Plan created the profile folder")
	}
}

func TestPlanIsEmptyWhenInSync(t *testing.T) {
	root, shared := sharedFixture(t)
	profileDir := filepath.Join(root, "acct")
	linkAll(t, profileDir, shared)

	if steps := mustPlan(t, profileDir, shared); len(steps) != 0 {
		t.Errorf("in-sync profile planned %v", steps)
	}
}

func TestPlanRepointsLinkAimedElsewhere(t *testing.T) {
	root, shared := sharedFixture(t)
	profileDir := filepath.Join(root, "acct")
	linkAll(t, profileDir, shared)
	other := filepath.Join(root, "other", "commands")
	mkdirT(t, other)
	if err := os.Remove(filepath.Join(profileDir, "commands")); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, other, filepath.Join(profileDir, "commands"))
	// right place, wrong name
	if err := os.Remove(filepath.Join(profileDir, "agents")); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, filepath.Join(shared, "projects"), filepath.Join(profileDir, "agents"))

	got := stepMap(mustPlan(t, profileDir, shared))

	if len(got) != 2 {
		t.Fatalf("planned %v, want commands and agents only", got)
	}
	for _, n := range []string{"commands", "agents"} {
		if got[n].Action != Repoint || got[n].Source != filepath.Join(shared, n) {
			t.Errorf("%s: %+v, want Repoint to %s", n, got[n], filepath.Join(shared, n))
		}
	}
}

func TestPlanAcceptsRelativeLinkToTheRightPlace(t *testing.T) {
	root, shared := sharedFixture(t)
	profileDir := filepath.Join(root, "acct")
	linkAll(t, profileDir, shared)
	if err := os.Remove(filepath.Join(profileDir, "commands")); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, filepath.Join("..", "shared", "commands"), filepath.Join(profileDir, "commands"))

	if steps := mustPlan(t, profileDir, shared); len(steps) != 0 {
		t.Errorf("relative link to the right place planned %v", steps)
	}
}

func TestPlanAcceptsLinkSpelledThroughASymlinkedHome(t *testing.T) {
	root, shared := sharedFixture(t)
	viaLink := filepath.Join(root, "home-link")
	mustSymlink(t, shared, viaLink)
	profileDir := filepath.Join(root, "acct")
	linkAll(t, profileDir, viaLink)

	if steps := mustPlan(t, profileDir, shared); len(steps) != 0 {
		t.Errorf("link through a symlinked home planned %v", steps)
	}
}

func TestPlanRemovesDanglingLinkWhoseSourceIsGone(t *testing.T) {
	root, shared := sharedFixture(t)
	profileDir := filepath.Join(root, "acct")
	linkAll(t, profileDir, shared)
	// a name that used to be shared and was deleted from the shared home
	mustSymlink(t, filepath.Join(shared, "hooks"), filepath.Join(profileDir, "hooks"))
	// the shared commands folder disappears too
	if err := os.RemoveAll(filepath.Join(shared, "commands")); err != nil {
		t.Fatal(err)
	}

	got := stepMap(mustPlan(t, profileDir, shared))

	if len(got) != 2 {
		t.Fatalf("planned %v, want hooks and commands only", got)
	}
	for _, n := range []string{"hooks", "commands"} {
		if got[n].Action != Remove || !strings.Contains(got[n].Why, "gone") {
			t.Errorf("%s: %+v, want Remove because the source is gone", n, got[n])
		}
	}
}

func TestPlanRemovesOldLinkForANameThatMustNotBeShared(t *testing.T) {
	root, shared := sharedFixture(t)
	profileDir := filepath.Join(root, "acct")
	linkAll(t, profileDir, shared)
	// an earlier design linked the sessions folder, and a stats file
	mustSymlink(t, filepath.Join(shared, "sessions"), filepath.Join(profileDir, "sessions"))
	mustSymlink(t, filepath.Join(shared, "stats-cache.json"), filepath.Join(profileDir, "stats-cache.json"))
	mustSymlink(t, filepath.Join(shared, ".credentials.json"), filepath.Join(profileDir, ".credentials.json"))

	got := stepMap(mustPlan(t, profileDir, shared))

	if len(got) != 3 {
		t.Fatalf("planned %v, want the three stale links only", got)
	}
	for _, n := range []string{"sessions", "stats-cache.json", ".credentials.json"} {
		if got[n].Action != Remove || !strings.Contains(got[n].Why, "shared name") {
			t.Errorf("%s: %+v, want Remove because it is not a shared name", n, got[n])
		}
	}
}

func TestPlanLeavesForeignLinksAlone(t *testing.T) {
	root, shared := sharedFixture(t)
	profileDir := filepath.Join(root, "acct")
	linkAll(t, profileDir, shared)
	mine := filepath.Join(root, "mine")
	mkdirT(t, mine)
	mustSymlink(t, mine, filepath.Join(profileDir, "mine"))
	mustSymlink(t, filepath.Join(root, "gone"), filepath.Join(profileDir, "broken"))
	// into the shared home, but deeper than anything clalt links
	mustSymlink(t, filepath.Join(shared, "commands", "inner"), filepath.Join(profileDir, "deep"))
	// the shared home itself
	mustSymlink(t, shared, filepath.Join(profileDir, "home"))

	if steps := mustPlan(t, profileDir, shared); len(steps) != 0 {
		t.Errorf("planned %v, want links clalt did not make left alone", steps)
	}
}

func TestPlanKeepsRealFilesAndFoldersInTheWay(t *testing.T) {
	root, shared := sharedFixture(t)
	profileDir := filepath.Join(root, "acct")
	linkAll(t, profileDir, shared)
	for _, n := range []string{"commands", "settings.json"} {
		if err := os.Remove(filepath.Join(profileDir, n)); err != nil {
			t.Fatal(err)
		}
	}
	mkdirT(t, filepath.Join(profileDir, "commands"))
	writeT(t, filepath.Join(profileDir, "commands", "mine.md"), "own")
	writeT(t, filepath.Join(profileDir, "settings.json"), "own")

	got := stepMap(mustPlan(t, profileDir, shared))

	if len(got) != 2 {
		t.Fatalf("planned %v, want commands and settings.json only", got)
	}
	for _, n := range []string{"commands", "settings.json"} {
		if got[n].Action != Keep || got[n].Why == "" {
			t.Errorf("%s: %+v, want Keep with a reason", n, got[n])
		}
	}
}

func TestPlanIgnoresPerAccountContent(t *testing.T) {
	root, shared := sharedFixture(t)
	profileDir := filepath.Join(root, "acct")
	linkAll(t, profileDir, shared)
	mkdirT(t, filepath.Join(profileDir, "sessions"))
	mkdirT(t, filepath.Join(profileDir, "cache"))
	writeT(t, filepath.Join(profileDir, ".credentials.json"), "own")
	writeT(t, filepath.Join(profileDir, ".claude.json"), "{}")
	writeT(t, filepath.Join(profileDir, "stats-cache.json"), "{}")

	if steps := mustPlan(t, profileDir, shared); len(steps) != 0 {
		t.Errorf("per-account content planned %v", steps)
	}
}
