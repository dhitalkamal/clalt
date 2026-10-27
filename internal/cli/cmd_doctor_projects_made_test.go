package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// the account still has the link that run made, and the folder it aims at went
// away. run makes the folder and the link is right again, so doctor has no
// reason to say that the history is out of reach
func TestDoctorTreatsALinkToAMissingSharedFolderAsSomethingRunMakesRight(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	shared := movedAwaySharedProjects(t, e)

	code, rep := doctor(t, e)

	links, _ := rep.find("work", "links")
	if code != 0 || links.Status != "warn" || strings.Contains(links.Detail, "refuses") ||
		!strings.Contains(links.Detail, "'clalt run work' makes "+shared) {
		t.Errorf("exit %d, links = %s %q", code, links.Status, links.Detail)
	}
	if exists(shared) {
		t.Error("doctor made the shared folder")
	}
	if code := e.run("run", "work"); code != 0 {
		t.Fatalf("run refused (%d): %s", code, e.errOut.String())
	}
	if _, rep := doctor(t, e); rep.Failed != 0 || rep.Warned != 0 {
		t.Errorf("after run: %d failed, %d warned: %+v", rep.Failed, rep.Warned, rep.Checks)
	}
}

// a file where the folder belongs is the account's own as much as a folder with
// something in it, and run refuses it the same way
func TestDoctorFailsForARealProjectsFileWhenTheSharedFolderIsMissing(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	projects := filepath.Join(e.folder("work"), "projects")
	os.Remove(projects)
	if err := os.WriteFile(projects, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	movedAwaySharedProjects(t, e)

	code, rep := doctor(t, e)

	rep.want(t, "work", "links", "fail", "a real file is in the way")
	if c, _ := rep.find("work", "links"); strings.Contains(c.Detail, "makes") || code != 1 {
		t.Errorf("exit %d: run cannot link over a file, so it does not make anything of this: %q", code, c.Detail)
	}
	if runCode := e.run("run", "work"); runCode != 1 || e.claude.sessionStarted() {
		t.Errorf("run exited %d, started %v: doctor and run have to agree", runCode, e.claude.sessionStarted())
	}
}

// doctor says what run is going to do and does none of it
func TestDoctorNeverMakesTheSharedProjectsFolderItReportsAsMissing(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	projects := filepath.Join(e.folder("work"), "projects")
	os.Remove(projects)
	mustMkdir(t, projects)
	movedAwaySharedProjects(t, e)
	before := snapshot(t, e.home)

	doctor(t, e)

	if after := snapshot(t, e.home); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Errorf("doctor changed the tree:\nbefore %v\nafter  %v", before, after)
	}
}

// the shared folder is not the only thing out of sync: the rest of what run
// repairs is listed next to what it does about the history
func TestDoctorListsTheOtherLinksNextToTheFolderThatRunMakes(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	if err := os.Remove(filepath.Join(e.folder("work"), "settings.json")); err != nil {
		t.Fatal(err)
	}
	movedAwaySharedProjects(t, e)

	_, rep := doctor(t, e)

	rep.want(t, "work", "links", "warn", "1 link to fix, 'clalt run work' does it (settings.json)")
	rep.want(t, "work", "links", "warn", "makes")
	if c, _ := rep.find("work", "links"); strings.Contains(c.Detail, "(projects)") {
		t.Errorf("projects is counted twice: %q", c.Detail)
	}
}

// with the shared folder there, an empty folder in the account holds nothing, so
// run replaces it with the link: a warning that run repairs and not a failure
func TestDoctorTreatsAnEmptyProjectsFolderInTheAccountAsALinkToFix(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	projects := filepath.Join(e.folder("work"), "projects")
	os.Remove(projects)
	mustMkdir(t, projects)

	code, rep := doctor(t, e)

	rep.want(t, "work", "links", "warn", "1 link to fix, 'clalt run work' does it (projects)")
	if c, _ := rep.find("work", "links"); code != 0 || strings.Contains(c.Detail, "not linked to the shared history") {
		t.Errorf("exit %d: %q", code, c.Detail)
	}
	if runCode := e.run("run", "work"); runCode != 0 {
		t.Fatalf("run refused (%d): %s", runCode, e.errOut.String())
	}
	if _, rep := doctor(t, e); rep.Failed != 0 || rep.Warned != 0 {
		t.Errorf("after run: %d failed, %d warned: %+v", rep.Failed, rep.Warned, rep.Checks)
	}
}
