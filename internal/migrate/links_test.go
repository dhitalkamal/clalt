package migrate

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSymlinkedProfileFolderIsReported(t *testing.T) {
	profiles, dest := dirs(t)
	away := filepath.Join(t.TempDir(), "away")
	put(t, away, "projects/-p/a.jsonl", "precious")
	symlink(t, away, filepath.Join(profiles, "work"))
	before := snapshot(t, away)

	rep := mustRun(t, profiles, dest, Options{})
	p := only(t, rep)
	if p.Name != "work" || p.Status != StatusLink || p.LinkOf != LinkOfProfile || p.Target != resolved(t, away) || p.Error != "" {
		t.Errorf("got %+v, want work reported as a profile link to %s", p, resolved(t, away))
	}
	if rep.Total.Linked != 1 || rep.Total.Copied != 0 {
		t.Errorf("total = %+v, want 1 linked and nothing copied", rep.Total)
	}
	if _, err := os.Lstat(dest); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("nothing was copied, so the destination should not exist: %v", err)
	}
	sameSnapshot(t, before, snapshot(t, away))
}

// a link to a plain file cannot be a profile folder, so it is not listed.
func TestRunLinkToAPlainFileIsNotAProfile(t *testing.T) {
	profiles, dest := dirs(t)
	file := put(t, t.TempDir(), "notes.txt", "x")
	symlink(t, file, filepath.Join(profiles, "notes"))
	put(t, profiles, "real/settings.json", "{}")

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Name != "real" {
		t.Errorf("got %+v, want only the real profile", p)
	}
}

func TestRunDanglingProfileLinkIsReportedWithTheReason(t *testing.T) {
	profiles, dest := dirs(t)
	gone := filepath.Join(t.TempDir(), "gone")
	symlink(t, gone, filepath.Join(profiles, "unplugged"))

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Status != StatusLink || p.LinkOf != LinkOfProfile || p.Target != gone {
		t.Errorf("got %+v, want a profile link to %s", p, gone)
	}
	if !strings.Contains(p.Error, "gone") {
		t.Errorf("error = %q, want the reason the link cannot be resolved", p.Error)
	}
}

func TestRunProjectsLinkNamesWhatIsLinked(t *testing.T) {
	profiles, dest := dirs(t)
	elsewhere := filepath.Join(t.TempDir(), "elsewhere")
	put(t, elsewhere, "-p/b.jsonl", "x")
	symlink(t, elsewhere, filepath.Join(profiles, "stray", "projects"))

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Status != StatusLink || p.LinkOf != LinkOfProjects || p.Error != "" {
		t.Errorf("got %+v, want a projects link that resolves", p)
	}
}

func TestRunBrokenProjectsLinkShowsWhyItCannotBeResolved(t *testing.T) {
	profiles, dest := dirs(t)
	gone := filepath.Join(t.TempDir(), "gone")
	symlink(t, gone, filepath.Join(profiles, "dangling", "projects"))

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.LinkOf != LinkOfProjects || p.Target != gone || !strings.Contains(p.Error, "gone") {
		t.Errorf("got %+v, want the broken projects link and the resolve error", p)
	}
}

func TestRunCountsEveryLinkItDoesNotFollow(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, dest, "-p/a.jsonl", "x")
	elsewhere := filepath.Join(t.TempDir(), "elsewhere")
	put(t, elsewhere, "-p/b.jsonl", "y")
	symlink(t, elsewhere, filepath.Join(profiles, "alpha", "projects"))
	symlink(t, elsewhere, filepath.Join(profiles, "beta"))
	symlink(t, dest, filepath.Join(profiles, "gamma", "projects"))
	symlink(t, filepath.Join(t.TempDir(), "gone"), filepath.Join(profiles, "delta", "projects"))

	rep := mustRun(t, profiles, dest, Options{})
	if len(rep.Profiles) != 4 {
		t.Fatalf("profiles = %+v, want 4", rep.Profiles)
	}
	// gamma links to the shared folder, which is not a link left alone
	if rep.Total.Linked != 3 {
		t.Errorf("linked = %d, want 3 (alpha, beta, delta)", rep.Total.Linked)
	}
}

func TestIsLink(t *testing.T) {
	for mode, want := range map[fs.FileMode]bool{
		fs.ModeSymlink:   true,
		fs.ModeIrregular: true, // a windows junction
		fs.ModeDir:       false,
		0:                false,
		fs.ModeNamedPipe: false,
	} {
		if got := isLink(mode); got != want {
			t.Errorf("isLink(%v) = %v, want %v", mode, got, want)
		}
	}
}
