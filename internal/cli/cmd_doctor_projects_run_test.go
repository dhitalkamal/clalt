package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// the shared home has no projects folder at all: claude ran and saved nothing,
// or it was moved away. run makes it and links the account, so doctor must not
// say that run refuses, and must not fail
func movedAwaySharedProjects(t *testing.T, e *testEnv) string {
	t.Helper()
	shared := filepath.Join(e.home, ".claude", "projects")
	if err := os.Rename(shared, filepath.Join(e.home, "projects.away")); err != nil {
		t.Fatal(err)
	}
	return shared
}

func TestDoctorJudgesAMissingSharedProjectsFolderTheWayRunDoes(t *testing.T) {
	cases := map[string]func(t *testing.T, e *testEnv, projects string){
		"the account has no projects entry": func(t *testing.T, e *testEnv, projects string) { os.Remove(projects) },
		"an empty real folder in the account": func(t *testing.T, e *testEnv, projects string) {
			os.Remove(projects)
			mustMkdir(t, projects)
		},
		"a link that aims somewhere else": func(t *testing.T, e *testEnv, projects string) {
			os.Remove(projects)
			mustSymlink(t, e.home, projects)
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			e.seedShared()
			e.signIn("work", "w@example.com")
			setup(t, e, filepath.Join(e.folder("work"), "projects"))
			shared := movedAwaySharedProjects(t, e)

			code, rep := doctor(t, e)

			links, _ := rep.find("work", "links")
			if code != 0 || links.Status != "warn" || strings.Contains(links.Detail, "refuses") ||
				!strings.Contains(links.Detail, "'clalt run work' makes "+shared) || !strings.Contains(links.Detail, "links the account") {
				t.Errorf("exit %d, links = %s %q: run makes the folder and links, doctor should say so", code, links.Status, links.Detail)
			}
			// and run does exactly that
			if runCode := e.run("run", "work"); runCode != 0 {
				t.Fatalf("run refused (%d): %s", runCode, e.errOut.String())
			}
			if _, rep := doctor(t, e); rep.Failed != 0 || rep.Warned != 0 {
				t.Errorf("after run: %d failed, %d warned: %+v", rep.Failed, rep.Warned, rep.Checks)
			}
		})
	}
}

// a real folder with something in it is the history of that account alone,
// whether or not the shared projects folder is there yet. that stays a failure.
func TestDoctorStillFailsForARealProjectsFolderWithContentWhenTheSharedOneIsMissing(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	projects := filepath.Join(e.folder("work"), "projects")
	os.Remove(projects)
	mustMkdir(t, filepath.Join(projects, "-mine"))
	movedAwaySharedProjects(t, e)

	code, rep := doctor(t, e)

	rep.want(t, "work", "links", "fail", "projects is not linked to the shared history")
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
}

// the shared projects folder that is there as a link into a volume that is not
// mounted is the unmounted volume, not a folder that run can make
func TestDoctorKeepsAnUnmountedSharedHistoryAsAWarningThatRunRefuses(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	unmountHistory(t, e)

	code, rep := doctor(t, e)

	rep.want(t, "work", "links", "warn", "is not reachable")
	rep.want(t, "work", "links", "warn", "'clalt run work' refuses")
	if c, _ := rep.find("work", "links"); strings.Contains(c.Detail, "makes") {
		t.Errorf("run cannot make a folder where a link to a missing volume is: %q", c.Detail)
	}
	if code != 0 {
		t.Errorf("exit %d, want 0", code)
	}
}

// the failure about projects is the headline, and the rest of what is out of
// sync is still there for whoever reads the line
func TestARealProjectsFolderDoesNotHideTheOtherFindings(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	folder := e.folder("work")
	projects := filepath.Join(folder, "projects")
	os.Remove(projects)
	mustMkdir(t, filepath.Join(projects, "-mine"))
	if err := os.Remove(filepath.Join(folder, "settings.json")); err != nil {
		t.Fatal(err)
	}

	_, rep := doctor(t, e)

	rep.want(t, "work", "links", "fail", "projects is not linked to the shared history")
	rep.want(t, "work", "links", "fail", "1 link to fix")
	rep.want(t, "work", "links", "fail", "settings.json")
}
