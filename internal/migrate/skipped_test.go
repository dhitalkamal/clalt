package migrate

import (
	"io/fs"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRunNamesTheEntriesItSkips(t *testing.T) {
	profiles, dest := dirs(t)
	outside := filepath.Join(t.TempDir(), "outside")
	put(t, outside, "secret.txt", "never copy me")
	put(t, outside, "dir/inner.txt", "nor me")
	root := filepath.Join(profiles, "work", "projects")
	put(t, root, "-p/real.jsonl", "keep")
	symlink(t, filepath.Join(outside, "secret.txt"), filepath.Join(root, "-p", "file-link.jsonl"))
	symlink(t, filepath.Join(outside, "dir"), filepath.Join(root, "-p", "dir-link"))
	symlink(t, "real.jsonl", filepath.Join(root, "-p", "rel-link.jsonl"))

	p := only(t, mustRun(t, profiles, dest, Options{}))
	want := []Skip{
		{Path: "-p/dir-link", Kind: "link", Target: filepath.Join(outside, "dir")},
		{Path: "-p/file-link.jsonl", Kind: "link", Target: filepath.Join(outside, "secret.txt")},
		{Path: "-p/rel-link.jsonl", Kind: "link", Target: "real.jsonl"},
	}
	if p.Skipped != 3 || !reflect.DeepEqual(p.SkippedEntries, want) {
		t.Errorf("got %d skipped %+v, want %+v", p.Skipped, p.SkippedEntries, want)
	}
}

func TestRunWithNothingSkippedListsNothing(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "x")

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Skipped != 0 || p.SkippedEntries == nil || len(p.SkippedEntries) != 0 {
		t.Errorf("got %d skipped %#v, want an empty list that is not nil", p.Skipped, p.SkippedEntries)
	}
}

func TestSkipKind(t *testing.T) {
	for mode, want := range map[fs.FileMode]string{
		fs.ModeSymlink:                    "link",
		fs.ModeNamedPipe:                  "pipe",
		fs.ModeSocket:                     "socket",
		fs.ModeDevice:                     "device",
		fs.ModeDevice | fs.ModeCharDevice: "device",
		fs.ModeIrregular:                  "irregular",
		fs.ModeSetuid:                     "special",
	} {
		if got := skipKind(mode); got != want {
			t.Errorf("skipKind(%v) = %q, want %q", mode, got, want)
		}
	}
}
