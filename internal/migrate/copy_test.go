package migrate

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunCopiesNestedTree(t *testing.T) {
	profiles, dest := dirs(t)
	files := map[string]string{
		"-Users-x-app/aaa.jsonl":                   "line one\nline two\n",
		"-Users-x-app/aaa/subagents/agent-1.jsonl": "sub",
		"-Users-x-app/memory/MEMORY.md":            "notes",
		"-Users-y-lib/bbb.jsonl":                   "",
	}
	var size int64
	for rel, body := range files {
		put(t, filepath.Join(profiles, "work", "projects"), rel, body)
		size += int64(len(body))
	}

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Status != StatusFolder || p.Copied != len(files) || p.Bytes != size {
		t.Errorf("got %+v, want %d files and %d bytes", p, len(files), size)
	}
	for rel, body := range files {
		got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(rel)))
		if err != nil || string(got) != body {
			t.Errorf("%s = %q (%v), want %q", rel, got, err, body)
		}
	}
	noTempFiles(t, dest)
}

func TestRunTotalsAddUpEveryProfile(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "a/projects/-p/one.jsonl", "123")
	put(t, profiles, "b/projects/-p/two.jsonl", "4567")

	rep := mustRun(t, profiles, dest, Options{})
	if rep.Total.Copied != 2 || rep.Total.Bytes != 7 {
		t.Errorf("total = %+v, want 2 files and 7 bytes", rep.Total)
	}
}

func TestRunKeepsModTime(t *testing.T) {
	profiles, dest := dirs(t)
	src := put(t, profiles, "work/projects/-p/old.jsonl", "x")
	when := time.Date(2025, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(src, when, when); err != nil {
		t.Fatal(err)
	}

	mustRun(t, profiles, dest, Options{})
	info, err := os.Stat(filepath.Join(dest, "-p", "old.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(when) {
		t.Errorf("mtime = %v, want %v", info.ModTime(), when)
	}
}

func TestRunCreatesPrivateFilesAndFolders(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits")
	}
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/deep/a.jsonl", "x") // the source is 0644 in 0755 folders

	mustRun(t, profiles, dest, Options{})
	for rel, want := range map[string]fs.FileMode{
		".":               0o700,
		"-p":              0o700,
		"-p/deep":         0o700,
		"-p/deep/a.jsonl": 0o600,
	} {
		info, err := os.Stat(filepath.Join(dest, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != want {
			t.Errorf("%s mode = %v, want %v", rel, info.Mode().Perm(), want)
		}
	}
}

func TestRunLeavesSourceUntouched(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "one")
	put(t, profiles, "work/projects/-p/sub/b.jsonl", "two")
	put(t, profiles, "work/settings.json", "{}")
	before := snapshot(t, profiles)

	mustRun(t, profiles, dest, Options{})
	sameSnapshot(t, before, snapshot(t, profiles))
}

func TestRunSkipsLinksInsideTheTree(t *testing.T) {
	profiles, dest := dirs(t)
	outside := filepath.Join(t.TempDir(), "outside")
	put(t, outside, "secret.txt", "never copy me")
	put(t, outside, "dir/inner.txt", "nor me")
	root := filepath.Join(profiles, "work", "projects")
	put(t, root, "-p/real.jsonl", "keep")
	symlink(t, filepath.Join(outside, "secret.txt"), filepath.Join(root, "-p", "file-link.jsonl"))
	symlink(t, filepath.Join(outside, "dir"), filepath.Join(root, "-p", "dir-link"))
	symlink(t, filepath.Join(root, "-p", "real.jsonl"), filepath.Join(root, "-p", "inner-link.jsonl"))

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Copied != 1 || p.Skipped != 3 {
		t.Errorf("got %d copied and %d skipped, want 1 and 3", p.Copied, p.Skipped)
	}
	got := snapshot(t, dest)
	for _, rel := range []string{".", "-p", filepath.Join("-p", "real.jsonl")} {
		delete(got, rel)
	}
	if len(got) != 0 {
		t.Errorf("only real.jsonl should reach the destination, also found %v", got)
	}
}

// the final name must not exist until the temp file holds everything, so a
// reader, or a crash, never sees half a transcript.
func TestRunPublishesOnlyCompleteFiles(t *testing.T) {
	profiles, dest := dirs(t)
	body := strings.Repeat("a transcript line\n", 5000)
	put(t, profiles, "work/projects/-p/big.jsonl", body)

	published := false
	opts := withFaults(faulty{link: func(s store, oldname, newname string) error {
		published = true
		if _, err := s.Lstat(newname); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("the final name exists before publishing: %v", err)
		}
		tmp, err := s.Open(oldname)
		if err != nil {
			t.Errorf("the temp file cannot be opened at publish time: %v", err)
			return s.Link(oldname, newname)
		}
		defer tmp.Close()
		got, err := io.ReadAll(tmp)
		if err != nil || string(got) != body {
			t.Errorf("the temp file was incomplete at publish time: %d bytes, %v", len(got), err)
		}
		return s.Link(oldname, newname)
	}})
	mustRun(t, profiles, dest, opts)

	if !published {
		t.Fatal("the copy never went through the link step")
	}
	got, err := os.ReadFile(filepath.Join(dest, "-p", "big.jsonl"))
	if err != nil || string(got) != body {
		t.Errorf("published file wrong: %d bytes, %v", len(got), err)
	}
	noTempFiles(t, dest)
}
