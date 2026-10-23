package migrate

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

var errBoom = errors.New("boom")

// closedSource hands back a source that was already closed, so the first thing
// done with it fails.
func closedSource(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	f.Close()
	return f, nil
}

func failFor(match func(name string) bool) func(s store, name string, flag int, perm fs.FileMode) (*os.File, error) {
	return func(s store, name string, flag int, perm fs.FileMode) (*os.File, error) {
		if match(name) {
			return nil, errBoom
		}
		return s.OpenFile(name, flag, perm)
	}
}

func isFinal(name string) bool { return !isTemp(name) }

// every step of a copy has to fail the file with its own reason, leave nothing
// under the final name and no temp file behind.
func TestRunReportsEachStepOfACopyThatFails(t *testing.T) {
	for name, c := range map[string]struct {
		opts Options
		want string
	}{
		"opening the source": {
			Options{openSrc: func(string) (*os.File, error) { return nil, errBoom }}, "boom"},
		"reading the size of the source": {
			Options{openSrc: closedSource}, "file already closed"},
		"making the folder": {
			withFaults(faulty{mkdirAll: func(store, string, fs.FileMode) error { return errBoom }}), "boom"},
		"creating the temp file": {
			withFaults(faulty{openFile: failFor(isTemp)}), "boom"},
		"copying into the temp file": {
			withFaults(faulty{openFile: readOnlyAfterCreate(isTemp)}), "bad file descriptor"},
		"setting the mtime": {
			withFaults(faulty{chtimes: func(store, string, time.Time, time.Time) error { return errBoom }}), "boom"},
		"linking": {
			withFaults(faulty{link: func(store, string, string) error { return errBoom }}), "boom"},
		"reopening the temp file for the fallback": {
			withFaults(faulty{link: noLinks, open: func(s store, name string) (*os.File, error) {
				if isTemp(name) {
					return nil, errBoom
				}
				return s.Open(name)
			}}), "boom"},
		"creating the file in place": {
			withFaults(faulty{link: noLinks, openFile: failFor(isFinal)}), "boom"},
		"copying in place": {
			withFaults(faulty{link: noLinks, openFile: readOnlyAfterCreate(isFinal)}), "bad file descriptor"},
		"setting the mtime in place": {
			withFaults(faulty{link: noLinks, chtimes: func(s store, name string, atime, mtime time.Time) error {
				if isFinal(name) {
					return errBoom
				}
				return s.Chtimes(name, atime, mtime)
			}}), "boom"},
	} {
		t.Run(name, func(t *testing.T) {
			profiles, dest := dirs(t)
			put(t, profiles, "work/projects/-p/a.jsonl", "mine")

			p := only(t, mustRun(t, profiles, dest, c.opts))
			if p.Copied != 0 || len(p.Failed) != 1 || p.Failed[0].Path != "-p/a.jsonl" ||
				p.Failed[0].Kind != KindFile || !strings.Contains(p.Failed[0].Error, c.want) {
				t.Errorf("got %+v, want a.jsonl failed with %q", p, c.want)
			}
			if _, err := os.Lstat(filepath.Join(dest, "-p", "a.jsonl")); err == nil {
				t.Error("nothing may be left under the final name")
			}
			noTempFiles(t, dest)
		})
	}
}

// what is already there is looked at and read before anything is written, and a
// failure on the way has to stop the file there, with the other side untouched.
func TestRunReportsAFailureToLookAtWhatIsThere(t *testing.T) {
	for name, c := range map[string]struct {
		opts Options
		want string
	}{
		"looking at it": {
			withFaults(faulty{lstat: func(store, string) (fs.FileInfo, error) {
				return nil, &os.PathError{Op: "lstat", Path: "x", Err: syscall.EACCES}
			}}), "permission denied"},
		"opening it": {
			withFaults(faulty{open: func(s store, name string) (*os.File, error) { return nil, errBoom }}), "boom"},
		"reading its size": {
			withFaults(faulty{open: func(s store, name string) (*os.File, error) {
				f, err := s.Open(name)
				if err == nil {
					f.Close()
				}
				return f, err
			}}), "file already closed"},
		"opening the source to compare": {
			Options{openSrc: func(string) (*os.File, error) { return nil, errBoom }}, "boom"},
		"reading the size of the source to compare": {
			Options{openSrc: closedSource}, "file already closed"},
	} {
		t.Run(name, func(t *testing.T) {
			profiles, dest := dirs(t)
			put(t, profiles, "work/projects/-p/a.jsonl", "mine")
			there := put(t, dest, "-p/a.jsonl", "mine")
			before := snapshot(t, dest)

			p := only(t, mustRun(t, profiles, dest, c.opts))
			if p.Copied != 0 || p.AlreadyThere != 0 || len(p.Conflicts) != 0 ||
				len(p.Failed) != 1 || !strings.Contains(p.Failed[0].Error, c.want) {
				t.Errorf("got %+v, want a.jsonl failed with %q", p, c.want)
			}
			if got, _ := os.ReadFile(there); string(got) != "mine" {
				t.Errorf("the file that was there changed to %q", got)
			}
			sameSnapshot(t, before, snapshot(t, dest))
		})
	}
}

// someone else had the name, and it was gone again when we looked: nothing can
// be judged, and nothing may be booked as a copy or a conflict.
func TestRunLostRaceThenVanishedIsAFailure(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")
	opts := withFaults(faulty{link: func(s store, oldname, newname string) error {
		return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: fs.ErrExist}
	}})

	p := only(t, mustRun(t, profiles, dest, opts))
	if p.Copied != 0 || len(p.Conflicts) != 0 || p.AlreadyThere != 0 || len(p.Failed) != 1 ||
		!strings.Contains(p.Failed[0].Error, errLostRace.Error()) {
		t.Errorf("got %+v, want a failure that says the destination appeared and vanished", p)
	}
	noTempFiles(t, dest)
}

// once links are known not to work, the in-place write is the one that can
// lose a race, and it must not replace what won.
func TestRunInPlaceWriteLosingTheRaceNeverOverwrites(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "first")
	put(t, profiles, "work/projects/-p/b.jsonl", "second")
	opts := withFaults(faulty{
		link: noLinks,
		openFile: func(s store, name string, flag int, perm fs.FileMode) (*os.File, error) {
			if filepath.Base(name) == "b.jsonl" {
				if err := os.WriteFile(filepath.Join(dest, name), []byte("theirs"), 0o600); err != nil {
					t.Error(err)
				}
			}
			return s.OpenFile(name, flag, perm)
		},
	})

	p := only(t, mustRun(t, profiles, dest, opts))
	if p.Copied != 1 || p.InPlace != 1 || len(p.Failed) != 0 ||
		len(p.Conflicts) != 1 || p.Conflicts[0].Path != "-p/b.jsonl" {
		t.Errorf("got %+v, want a.jsonl copied in place and b.jsonl a conflict", p)
	}
	if got, _ := os.ReadFile(filepath.Join(dest, "-p", "b.jsonl")); string(got) != "theirs" {
		t.Errorf("the file that won the race was replaced by %q", got)
	}
	noTempFiles(t, dest)
}

// the shared folder is made on the first write, and it can refuse.
func TestRunReportsASharedFolderItCannotCreate(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	readOnly(t, parent)

	p := only(t, mustRun(t, profiles, dest, Options{}))
	if p.Copied != 0 || len(p.Failed) != 1 || !strings.Contains(p.Failed[0].Error, "permission denied") {
		t.Errorf("got %+v, want a.jsonl failed with permission denied", p)
	}
}
