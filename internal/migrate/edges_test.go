package migrate

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReportsAProfilesFolderItCannotRead(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/settings.json", "{}")
	lockOut(t, profiles)

	if _, err := Run(profiles, dest, Options{}); err == nil {
		t.Error("a profiles folder that cannot be read should be an error")
	}
}

func TestResolveExistingReportsALinkLoop(t *testing.T) {
	root := t.TempDir()
	symlink(t, filepath.Join(root, "b"), filepath.Join(root, "a"))
	symlink(t, filepath.Join(root, "a"), filepath.Join(root, "b"))

	if _, err := ResolveExisting(filepath.Join(root, "a", "x")); err == nil {
		t.Error("a loop of links should be an error")
	}
}

func TestRunDryRunReportsASourceItCannotStat(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")

	p := only(t, mustRun(t, profiles, dest, Options{DryRun: true, openSrc: closedSource}))
	if p.Copied != 0 || len(p.Failed) != 1 || !strings.Contains(p.Failed[0].Error, closedText) {
		t.Errorf("got %+v, want a.jsonl failed in the dry run too", p)
	}
}

// the shared folder can be there and still refuse to be opened.
func TestRunReportsASharedFolderItCannotOpen(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")
	put(t, dest, "-other/x.jsonl", "there")
	writeOnly(t, dest)

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Copied != 0 || len(p.Failed) != 1 || !strings.Contains(p.Failed[0].Error, "permission denied") {
		t.Errorf("got %+v, want a.jsonl failed with permission denied", p)
	}
}
