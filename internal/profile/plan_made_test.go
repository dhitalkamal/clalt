package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// a shared home with everything but a projects folder: claude ran and saved
// nothing yet, or the folder was moved away.
func sharedWithoutProjects(t *testing.T) (root, shared string) {
	t.Helper()
	root, shared = sharedFixture(t)
	if err := os.Remove(filepath.Join(shared, "projects")); err != nil {
		t.Fatal(err)
	}
	return root, shared
}

// the caller is about to make the folder, so the account is judged against a
// home that has it: whatever the sync would do about the name afterwards is what
// the plan says
func TestPlanWithFoldersJudgesAMissingFolderAsIfItWereThere(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(t *testing.T, acct, shared string)
		action Action
		why    string
	}{
		{"no entry", func(t *testing.T, acct, shared string) {}, Link, "not linked yet"},
		{"a link that aims elsewhere", func(t *testing.T, acct, shared string) {
			mustSymlink(t, filepath.Dir(shared), filepath.Join(acct, "projects"))
		}, Repoint, "aims somewhere else"},
		{"an empty folder", func(t *testing.T, acct, shared string) {
			mkdirT(t, filepath.Join(acct, "projects"))
		}, Link, "empty folder"},
		{"a folder with something in it", func(t *testing.T, acct, shared string) {
			writeT(t, filepath.Join(acct, "projects", "-mine", "a.jsonl"), "{}")
		}, Keep, "real folder is in the way"},
		{"a real file", func(t *testing.T, acct, shared string) {
			writeT(t, filepath.Join(acct, "projects"), "x")
		}, Keep, "real file is in the way"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, shared := sharedWithoutProjects(t)
			acct := filepath.Join(root, "acct")
			mkdirT(t, acct)
			c.setup(t, acct, shared)

			steps, err := PlanWithFolders(acct, shared, "projects")
			if err != nil {
				t.Fatal(err)
			}

			st, ok := stepMap(steps)["projects"]
			if !ok || st.Action != c.action || st.Source != filepath.Join(shared, "projects") || !strings.Contains(st.Why, c.why) {
				t.Errorf("projects = %+v (planned: %v), want %v because %q", st, ok, c.action, c.why)
			}
		})
	}
}

// the link that run made before the folder went away is right once the folder is
// made again. a plain plan takes it for a link whose source is gone.
func TestPlanWithFoldersLeavesALinkToTheFolderThatWillBeMade(t *testing.T) {
	root, shared := sharedWithoutProjects(t)
	acct := filepath.Join(root, "acct")
	mustSymlink(t, filepath.Join(shared, "projects"), filepath.Join(acct, "projects"))

	plain := stepMap(mustPlan(t, acct, shared))
	steps, err := PlanWithFolders(acct, shared, "projects")
	if err != nil {
		t.Fatal(err)
	}

	if plain["projects"].Action != Remove {
		t.Fatalf("without the folder the plan is %+v, which this test is not about", plain["projects"])
	}
	if st, ok := stepMap(steps)["projects"]; ok {
		t.Errorf("planned %+v for a link that will be right", st)
	}
}

// only a name that is missing is made up: one the home has, in any form, is left
// as it is, and the rest of the plan does not change
func TestPlanWithFoldersLeavesWhatTheHomeHasAlone(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	mkdirT(t, filepath.Join(acct, "projects"))
	writeT(t, filepath.Join(acct, "projects", "mine.jsonl"), "{}")

	plain := mustPlan(t, acct, shared)
	steps, err := PlanWithFolders(acct, shared, "projects")
	if err != nil {
		t.Fatal(err)
	}

	if strings.Join(stepNames(steps), ",") != strings.Join(stepNames(plain), ",") || stepMap(steps)["projects"] != stepMap(plain)["projects"] {
		t.Errorf("planned %+v, want what Plan says: %+v", steps, plain)
	}
}

// a name that is not linked whole has no folder to make up: the plan would be a
// guess, so it is an error
func TestPlanWithFoldersRefusesANameThatIsNotLinkedWhole(t *testing.T) {
	for _, name := range []string{"skills", "plugins", "sessions", ".credentials.json"} {
		t.Run(name, func(t *testing.T) {
			root, shared := sharedWithoutProjects(t)

			steps, err := PlanWithFolders(filepath.Join(root, "acct"), shared, name)

			if err == nil || steps != nil {
				t.Errorf("PlanWithFolders(%q) = %v, %v: want an error", name, steps, err)
			}
		})
	}
}

// the plan is for doctor, which never writes
func TestPlanWithFoldersWritesNothing(t *testing.T) {
	root, shared := sharedWithoutProjects(t)
	acct := filepath.Join(root, "acct")
	mkdirT(t, filepath.Join(acct, "projects"))
	before := listTree(t, root)

	if _, err := PlanWithFolders(acct, shared, "projects"); err != nil {
		t.Fatal(err)
	}

	if after := listTree(t, root); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Errorf("the tree changed:\nbefore %v\nafter  %v", before, after)
	}
}

// a home that is not there is still the error that Plan gives, since there is
// nothing to make a folder in
func TestPlanWithFoldersNeedsTheHome(t *testing.T) {
	root := t.TempDir()

	_, err := PlanWithFolders(filepath.Join(root, "acct"), filepath.Join(root, "no-home"), "projects")

	if err == nil || !strings.Contains(err.Error(), ErrNoSharedHome.Error()) {
		t.Errorf("err = %v, want %v", err, ErrNoSharedHome)
	}
}
