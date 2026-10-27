package migrate

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInside(t *testing.T) {
	for _, c := range []struct {
		dir, path string
		want      bool
	}{
		{"/a/b", "/a/b", true},
		{"/a/b", "/a/b/c/d.jsonl", true},
		{"/a/b", "/a/b/..hidden", true},
		{"/a/b", "/a/b/c/../d", true},
		{"/a/b", "/a/bc", false},
		{"/a/b", "/a", false},
		{"/a/b", "/a/b/../c", false},
		{"/a/b", "/x/y", false},
		{"/a/b", "a/b/c", false},
	} {
		dir, path := filepath.FromSlash(c.dir), filepath.FromSlash(c.path)
		if got := Inside(dir, path); got != c.want {
			t.Errorf("Inside(%q, %q) = %v, want %v", c.dir, c.path, got, c.want)
		}
	}
}

// a folder that does not exist yet still has to be judged by where its
// existing part really leads.
func TestResolveExistingKeepsTheMissingTail(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	symlink(t, realDir, filepath.Join(root, "alias"))

	got, err := ResolveExisting(filepath.Join(root, "alias", "not", "there"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(resolved(t, realDir), "not", "there"); got != want {
		t.Errorf("ResolveExisting = %q, want %q", got, want)
	}
}

// a link inside the shared folder that leads elsewhere must not carry a copy
// out of it, in a real run or in the dry run that predicts it.
func TestRunNeverWritesThroughALinkedDestinationFolder(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	symlink(t, outside, filepath.Join(dest, "-p"))
	before := snapshot(t, outside)

	for _, dry := range []bool{true, false} {
		p := only(t, mustRun(t, profiles, dest, Options{DryRun: dry}))
		if p.Copied != 0 || len(p.Failed) != 1 || p.Failed[0].Path != "-p/a.jsonl" || p.Failed[0].Kind != KindFile {
			t.Errorf("dry=%v: got %+v, want the file a.jsonl failed", dry, p)
		}
		sameSnapshot(t, before, snapshot(t, outside))
	}
}

func TestRunCopiesIntoADestinationThatIsALink(t *testing.T) {
	profiles, _ := dirs(t)
	realDir := filepath.Join(t.TempDir(), "real-projects")
	if err := os.MkdirAll(realDir, 0o700); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "projects")
	symlink(t, realDir, dest)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Copied != 1 || len(p.Failed) != 0 {
		t.Fatalf("got %+v, want 1 copied", p)
	}
	if got, err := os.ReadFile(filepath.Join(realDir, "-p", "a.jsonl")); err != nil || string(got) != "mine" {
		t.Errorf("the file should land in the real folder: %q, %v", got, err)
	}
}

func TestRunRejectsADestinationThatIsAFile(t *testing.T) {
	profiles, _ := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")
	dest := put(t, t.TempDir(), "projects", "not a folder")

	if _, err := Run(profiles, dest, Options{}); err == nil {
		t.Error("a destination that is a file should be refused")
	}
}

// when a profile's projects folder is the shared folder itself (say ~/.claude is
// linked to a profile), copying it onto itself would be pointless.
func TestRunProjectsFolderThatIsTheDestinationIsShared(t *testing.T) {
	profiles, _ := dirs(t)
	dest := filepath.Join(profiles, "work", "projects")
	put(t, dest, "-p/a.jsonl", "mine")
	before := snapshot(t, dest)

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Status != StatusShared || p.Copied != 0 {
		t.Errorf("got %+v, want status %q", p, StatusShared)
	}
	sameSnapshot(t, before, snapshot(t, dest))
}

// the profiles folder is reached through an alias while the destination is
// spelled plainly, so only resolving the profile's side says they match.
func TestRunProjectsFolderReachedThroughAnAliasIsShared(t *testing.T) {
	profiles, _ := dirs(t)
	dest := filepath.Join(profiles, "work", "projects")
	put(t, dest, "-p/a.jsonl", "mine")
	alias := filepath.Join(t.TempDir(), "alias")
	symlink(t, profiles, alias)

	p := only(t, mustRun(t, alias, dest, Options{}))
	if p.Status != StatusShared || p.Copied != 0 || p.AlreadyThere != 0 {
		t.Errorf("got %+v, want status %q with nothing counted", p, StatusShared)
	}
}

// a name that exists but leads nowhere is not a folder that is merely missing,
// and treating it as missing made a dry run promise a copy that then failed.
func TestResolveExistingNamesADanglingLink(t *testing.T) {
	root := t.TempDir()
	gone := filepath.Join(root, "unplugged")
	symlink(t, filepath.Join(root, "nowhere"), gone)

	for _, path := range []string{gone, filepath.Join(gone, "not", "there")} {
		_, err := ResolveExisting(path)
		if err == nil || !strings.Contains(err.Error(), "leads nowhere") || !strings.Contains(err.Error(), gone) {
			t.Errorf("ResolveExisting(%q) err = %v, want the link named as one that leads nowhere", path, err)
		}
	}
}

func TestRunDanglingDestinationIsRefusedInADryRunToo(t *testing.T) {
	for _, dry := range []bool{true, false} {
		profiles, dest := dirs(t)
		put(t, profiles, "work/projects/-p/a.jsonl", "mine")
		symlink(t, filepath.Join(t.TempDir(), "unplugged"), dest)

		_, err := Run(profiles, dest, Options{DryRun: dry})
		if err == nil || !strings.Contains(err.Error(), "leads nowhere") {
			t.Errorf("dry=%v: err = %v, want a dangling link error", dry, err)
		}
	}
}

func TestRunDanglingFolderUnderTheDestinationFailsItsFiles(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")
	put(t, profiles, "work/projects/-ok/b.jsonl", "fine")
	if err := os.MkdirAll(dest, 0o700); err != nil {
		t.Fatal(err)
	}
	symlink(t, filepath.Join(t.TempDir(), "unplugged"), filepath.Join(dest, "-p"))

	var reports []Report
	for _, dry := range []bool{true, false} {
		rep := mustRun(t, profiles, dest, Options{DryRun: dry})
		p := only(t, rep)
		if p.Copied != 1 || len(p.Failed) != 1 || p.Failed[0].Path != "-p/a.jsonl" ||
			!strings.Contains(p.Failed[0].Error, "leads nowhere") {
			t.Errorf("dry=%v: got %+v, want a.jsonl failed as a link that leads nowhere", dry, p)
		}
		reports = append(reports, rep)
	}
	reports[0].DryRun = false
	if !reflect.DeepEqual(reports[0], reports[1]) {
		t.Errorf("dry run and real run disagree:\n  dry  %+v\n  real %+v", reports[0], reports[1])
	}
}
