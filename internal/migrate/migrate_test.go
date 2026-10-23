package migrate

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestRunProfileWithoutProjects(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "bare/settings.json", "{}")

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Name != "bare" || p.Status != StatusNothing {
		t.Errorf("got %+v, want bare with status %q", p, StatusNothing)
	}
	if _, err := os.Lstat(dest); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the destination should not be created for an empty profile: %v", err)
	}
}

func TestRunProjectsThatIsAFileHasNothingToCopy(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "odd/projects", "not a folder")

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Status != StatusNothing {
		t.Errorf("status = %q, want %q", p.Status, StatusNothing)
	}
}

func TestRunSymlinkToDestinationIsAlreadyShared(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, dest, "-proj/a.jsonl", "one")
	symlink(t, dest, filepath.Join(profiles, "linked", "projects"))
	before := snapshot(t, dest)

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Status != StatusShared {
		t.Errorf("status = %q, want %q", p.Status, StatusShared)
	}
	if p.Copied != 0 || p.AlreadyThere != 0 || len(p.Failed) != 0 {
		t.Errorf("a shared profile has nothing to count: %+v", p)
	}
	sameSnapshot(t, before, snapshot(t, dest))
}

// the destination is spelled through an alias while the profile's link leads to
// the real folder, so only resolving both sides says they match.
func TestRunSharedLinkIsComparedByResolvedPlace(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, dest, "-proj/a.jsonl", "one")
	alias := filepath.Join(t.TempDir(), "alias")
	symlink(t, dest, alias)
	symlink(t, dest, filepath.Join(profiles, "linked", "projects"))

	p := only(t, mustRun(t, profiles, alias, Options{}))
	if p.Status != StatusShared {
		t.Errorf("status = %q, want %q", p.Status, StatusShared)
	}
}

// the other way round: the link runs through an alias, the destination is plain.
func TestRunLinkRoutedThroughAnAliasIsShared(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, dest, "-proj/a.jsonl", "one")
	alias := filepath.Join(t.TempDir(), "alias")
	symlink(t, dest, alias)
	symlink(t, alias, filepath.Join(profiles, "linked", "projects"))

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Status != StatusShared {
		t.Errorf("status = %q, want %q", p.Status, StatusShared)
	}
}

func TestRunSymlinkElsewhereIsNotFollowed(t *testing.T) {
	profiles, dest := dirs(t)
	elsewhere := filepath.Join(t.TempDir(), "elsewhere")
	put(t, elsewhere, "-proj/b.jsonl", "do not touch")
	symlink(t, elsewhere, filepath.Join(profiles, "stray", "projects"))
	before := snapshot(t, elsewhere)

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Status != StatusLink || p.Target != resolved(t, elsewhere) {
		t.Errorf("got status %q target %q, want %q to %q", p.Status, p.Target, StatusLink, resolved(t, elsewhere))
	}
	if p.Copied != 0 {
		t.Errorf("a link elsewhere must not be copied: %+v", p)
	}
	if _, err := os.Lstat(dest); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("nothing was copied, so the destination should not exist: %v", err)
	}
	sameSnapshot(t, before, snapshot(t, elsewhere))
}

func TestRunBrokenLinkIsReportedAsALink(t *testing.T) {
	profiles, dest := dirs(t)
	gone := filepath.Join(t.TempDir(), "gone")
	symlink(t, gone, filepath.Join(profiles, "dangling", "projects"))

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Status != StatusLink || p.Target != gone {
		t.Errorf("got status %q target %q, want %q to %q", p.Status, p.Target, StatusLink, gone)
	}
}

func TestRunRealProjectsFolderIsScanned(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-proj/a.jsonl", "one")

	p := only(t, mustRun(t, profiles, dest, Options{DryRun: true}))
	if p.Status != StatusFolder {
		t.Errorf("status = %q, want %q", p.Status, StatusFolder)
	}
}

func TestRunWithoutProfilesFolderReportsNothing(t *testing.T) {
	_, dest := dirs(t)
	rep := mustRun(t, filepath.Join(t.TempDir(), "profiles"), dest, Options{})
	if len(rep.Profiles) != 0 {
		t.Errorf("got profiles %+v, want none", rep.Profiles)
	}
}

func TestRunScansOnlyFoldersInNameOrder(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "zeta/settings.json", "{}")
	put(t, profiles, "notes.txt", "a stray file is not a profile")
	put(t, profiles, "alpha/settings.json", "{}")

	rep := mustRun(t, profiles, dest, Options{})
	var names []string
	for _, p := range rep.Profiles {
		names = append(names, p.Name)
	}
	if want := []string{"alpha", "zeta"}; !reflect.DeepEqual(names, want) {
		t.Errorf("profiles = %v, want %v", names, want)
	}
}

func TestRunRejectsPathsThatAreNotAbsolute(t *testing.T) {
	abs := t.TempDir()
	for _, c := range []struct{ profiles, dest string }{
		{"profiles", abs},
		{abs, "projects"},
		{abs, ""},
	} {
		_, err := Run(c.profiles, c.dest, Options{})
		if err == nil {
			t.Errorf("Run(%q, %q) should fail", c.profiles, c.dest)
			continue
		}
		// a user with HOME unset has to see which path was the problem
		for _, p := range []string{c.profiles, c.dest} {
			if !strings.Contains(err.Error(), strconv.Quote(p)) {
				t.Errorf("Run(%q, %q) error %q does not name %q", c.profiles, c.dest, err, p)
			}
		}
	}
}
