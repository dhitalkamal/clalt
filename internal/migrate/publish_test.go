package migrate

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

// another process creating the file between our look and our link must win:
// the link fails, and what is there is judged again, never replaced.
func TestRunLosingTheRaceNeverOverwrites(t *testing.T) {
	for name, c := range map[string]struct {
		theirs   string
		conflict bool
	}{
		"different bytes": {"theirs", true},
		"same bytes":      {"mine", false},
	} {
		t.Run(name, func(t *testing.T) {
			profiles, dest := dirs(t)
			put(t, profiles, "work/projects/-p/a.jsonl", "mine")
			opts := withFaults(faulty{link: func(s store, oldname, newname string) error {
				if err := os.WriteFile(filepath.Join(dest, newname), []byte(c.theirs), 0o600); err != nil {
					t.Error(err)
				}
				return s.Link(oldname, newname)
			}})

			p := only(t, mustRun(t, profiles, dest, opts))
			if c.conflict && !reflect.DeepEqual(p.Conflicts, []Conflict{{"-p/a.jsonl", ReasonDiffers}}) {
				t.Errorf("conflicts = %v, want a.jsonl", p.Conflicts)
			}
			if !c.conflict && p.AlreadyThere != 1 {
				t.Errorf("got %+v, want 1 already there", p)
			}
			if p.Copied != 0 || len(p.Failed) != 0 {
				t.Errorf("got %+v, want nothing copied and nothing failed", p)
			}
			got, _ := os.ReadFile(filepath.Join(dest, "-p", "a.jsonl"))
			if string(got) != c.theirs {
				t.Errorf("the file that won the race was replaced by %q", got)
			}
			noTempFiles(t, dest)
		})
	}
}

func noLinks(s store, oldname, newname string) error {
	return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: errors.ErrUnsupported}
}

func TestRunFallsBackWhenLinksAreUnsupported(t *testing.T) {
	profiles, dest := dirs(t)
	src := put(t, profiles, "work/projects/-p/a.jsonl", "mine")
	when := time.Date(2025, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(src, when, when); err != nil {
		t.Fatal(err)
	}

	tried := false
	p := only(t, mustRun(t, profiles, dest, withFaults(faulty{link: func(s store, o, n string) error {
		tried = true
		return noLinks(s, o, n)
	}})))
	if !tried {
		t.Fatal("the link was never tried, so the fallback was not exercised")
	}
	if p.Copied != 1 || p.Bytes != 4 || len(p.Failed) != 0 {
		t.Fatalf("got %+v, want 1 file copied", p)
	}
	final := filepath.Join(dest, "-p", "a.jsonl")
	got, err := os.ReadFile(final)
	if err != nil || string(got) != "mine" {
		t.Errorf("content = %q (%v)", got, err)
	}
	info, _ := os.Stat(final)
	if !info.ModTime().Equal(when) {
		t.Errorf("mtime = %v, want %v", info.ModTime(), when)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
	noTempFiles(t, dest)
}

func TestRunFallbackNeverOverwritesEither(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")
	opts := withFaults(faulty{link: func(s store, oldname, newname string) error {
		if err := os.WriteFile(filepath.Join(dest, newname), []byte("theirs"), 0o600); err != nil {
			t.Error(err)
		}
		return noLinks(s, oldname, newname)
	}})

	p := only(t, mustRun(t, profiles, dest, opts))
	if !reflect.DeepEqual(p.Conflicts, []Conflict{{"-p/a.jsonl", ReasonDiffers}}) || p.Copied != 0 {
		t.Errorf("got %+v, want a conflict", p)
	}
	got, _ := os.ReadFile(filepath.Join(dest, "-p", "a.jsonl"))
	if string(got) != "theirs" {
		t.Errorf("the file that was there first was replaced by %q", got)
	}
	noTempFiles(t, dest)
}

// the fallback writes straight to the final name, so on any error it has to
// remove what it started.
func TestWriteInPlaceRemovesAPartialFile(t *testing.T) {
	root, dir := openDest(t)
	r := io.MultiReader(strings.NewReader("half of it"), iotest.ErrReader(errors.New("the disk went away")))

	c := &copier{destReal: dir}
	if _, err := c.writeInPlace(root, "a.jsonl", r, time.Now()); err == nil {
		t.Fatal("the read error should come back")
	}
	if _, err := os.Lstat(filepath.Join(dir, "a.jsonl")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a partial file was left under the final name: %v", err)
	}
}

// the file that stopped it is not ours to clean up.
func TestWriteInPlaceKeepsAFileItDidNotCreate(t *testing.T) {
	root, dir := openDest(t)
	existing := put(t, dir, "a.jsonl", "already here")

	c := &copier{destReal: dir}
	_, err := c.writeInPlace(root, "a.jsonl", strings.NewReader("new"), time.Now())
	if !errors.Is(err, fs.ErrExist) {
		t.Errorf("err = %v, want one that says it exists", err)
	}
	if got, _ := os.ReadFile(existing); string(got) != "already here" {
		t.Errorf("the existing file now holds %q", got)
	}
}

// the containment check runs before the write, and a link swapped in between
// must still not carry the write out of the shared folder. the folder is opened
// once as a root, and the root refuses any name that leaves it.
func TestRunCannotBeLedOutByALinkSwappedInBeforeTheFolderIsMade(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	swapped := false
	opts := withFaults(faulty{mkdirAll: func(s store, name string, perm fs.FileMode) error {
		swapped = true
		symlink(t, outside, filepath.Join(dest, name))
		return s.MkdirAll(name, perm)
	}})

	p := only(t, mustRun(t, profiles, dest, opts))
	if !swapped {
		t.Fatal("the folder was never made through the destination root")
	}
	if p.Copied != 0 || len(p.Failed) != 1 || p.Failed[0].Path != "-p/a.jsonl" {
		t.Errorf("got %+v, want a.jsonl failed", p)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Errorf("a write escaped into %s: %v", outside, entries)
	}
}

func TestRunCannotBeLedOutByAFolderSwappedForALinkBeforeTheLink(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	swapped := false
	opts := withFaults(faulty{link: func(s store, oldname, newname string) error {
		swapped = true
		folder := filepath.Join(dest, filepath.Dir(newname))
		if err := os.Rename(folder, folder+"-moved"); err != nil {
			t.Fatal(err)
		}
		symlink(t, outside, folder)
		return s.Link(oldname, newname)
	}})

	p := only(t, mustRun(t, profiles, dest, opts))
	if !swapped {
		t.Fatal("the copy never went through the destination root")
	}
	if p.Copied != 0 || len(p.Failed) != 1 || p.Failed[0].Path != "-p/a.jsonl" {
		t.Errorf("got %+v, want a.jsonl failed", p)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Errorf("a write escaped into %s: %v", outside, entries)
	}
}

func TestRunCannotBeLedOutByAFolderSwappedBeforeTheTempFile(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	swapped := false
	opts := withFaults(faulty{mkdirAll: func(s store, name string, perm fs.FileMode) error {
		err := s.MkdirAll(name, perm)
		swapped = true
		folder := filepath.Join(dest, name)
		if rerr := os.Rename(folder, folder+"-moved"); rerr != nil {
			t.Fatal(rerr)
		}
		symlink(t, outside, folder)
		return err
	}})

	p := only(t, mustRun(t, profiles, dest, opts))
	if !swapped {
		t.Fatal("the folder was never made through the destination root")
	}
	if p.Copied != 0 || len(p.Failed) != 1 {
		t.Errorf("got %+v, want the file to fail", p)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Errorf("a write escaped into %s: %v", outside, entries)
	}
}
