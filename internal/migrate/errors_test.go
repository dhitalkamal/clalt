package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunKeepsGoingAfterAFileFails(t *testing.T) {
	profiles, dest := dirs(t)
	root := filepath.Join(profiles, "work", "projects")
	bad := put(t, root, "-p/a-bad.jsonl", "private words")
	put(t, root, "-p/b-good.jsonl", "fine")
	lockOut(t, bad)

	rep := mustRun(t, profiles, dest, Options{})
	p := only(t, rep)
	if p.Copied != 1 || len(p.Failed) != 1 || p.Failed[0].Path != "-p/a-bad.jsonl" {
		t.Fatalf("got %+v, want 1 copied and a-bad.jsonl failed", p)
	}
	if p.Failed[0].Kind != KindFile {
		t.Errorf("kind = %q, want %q", p.Failed[0].Kind, KindFile)
	}
	if rep.Total.Failed != 1 {
		t.Errorf("total failed = %d, want 1", rep.Total.Failed)
	}
	if strings.Contains(p.Failed[0].Error, "private words") {
		t.Errorf("an error must never carry file contents: %q", p.Failed[0].Error)
	}
	if _, err := os.Stat(filepath.Join(dest, "-p", "b-good.jsonl")); err != nil {
		t.Errorf("the good file should still be copied: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dest, "-p", "a-bad.jsonl")); err == nil {
		t.Error("the failed file must not appear in the destination")
	}
	noTempFiles(t, dest)
}

func TestRunReportsAFolderItCannotRead(t *testing.T) {
	profiles, dest := dirs(t)
	root := filepath.Join(profiles, "work", "projects")
	put(t, root, "-ok/a.jsonl", "fine")
	put(t, root, "-locked/b.jsonl", "hidden")
	lockOut(t, filepath.Join(root, "-locked"))

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Copied != 1 || len(p.Failed) != 1 || p.Failed[0].Path != "-locked" || p.Failed[0].Kind != KindFolder {
		t.Errorf("got %+v, want 1 copied and the folder -locked failed", p)
	}
}

func TestRunReportsAProjectsFolderItCannotRead(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "fine")
	lockOut(t, filepath.Join(profiles, "work", "projects"))

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if len(p.Failed) != 1 || p.Failed[0].Path != "projects" || p.Failed[0].Kind != KindFolder {
		t.Errorf("got %+v, want the folder projects failed", p)
	}
}

// a profile folder that cannot be searched hides its projects folder, which is
// not the same as having none.
func TestRunReportsAProfileItCannotRead(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "fine")
	lockOut(t, filepath.Join(profiles, "work"))

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Status != StatusFolder || len(p.Failed) != 1 || p.Failed[0].Path != "projects" || p.Failed[0].Kind != KindFolder {
		t.Errorf("got %+v, want a failed projects folder", p)
	}
}
