package migrate

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// cannotRemove says no to removing the names it is asked about, and leaves the
// file where it is.
func cannotRemove(match func(name string) bool, next func(s store, name string) error) func(store, string) error {
	return func(s store, name string) error {
		if match(name) {
			return &os.PathError{Op: "remove", Path: name, Err: syscall.EACCES}
		}
		if next != nil {
			return next(s, name)
		}
		return s.Remove(name)
	}
}

func isTemp(name string) bool { return strings.HasPrefix(filepath.Base(name), tempPrefix) }

// readOnlyAfterCreate makes the write that follows fail, by handing back the
// file it just created opened for reading only.
func readOnlyAfterCreate(match func(name string) bool) func(s store, name string, flag int, perm fs.FileMode) (*os.File, error) {
	return func(s store, name string, flag int, perm fs.FileMode) (*os.File, error) {
		f, err := s.OpenFile(name, flag, perm)
		if err != nil || !match(name) {
			return f, err
		}
		f.Close()
		return s.OpenFile(name, os.O_RDONLY, 0)
	}
}

func leftoverTemps(t *testing.T, dir string) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && isTemp(path) {
			found = append(found, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// the file is in place, so it counts as copied, and the temp file that could
// not be removed is a failure the user has to hear about.
func TestRunReportsATempFileItCannotRemove(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")
	opts := withFaults(faulty{remove: cannotRemove(isTemp, nil)})

	p := only(t, mustRun(t, profiles, dest, opts))
	left := leftoverTemps(t, dest)
	if p.Copied != 1 || len(p.Failed) != 1 || len(left) != 1 {
		t.Fatalf("got %+v with leftovers %v, want 1 copied and 1 failure for the temp file", p, left)
	}
	msg := p.Failed[0].Error
	if !strings.Contains(msg, "could not remove the temp file") || !strings.Contains(msg, filepath.Base(left[0])) {
		t.Errorf("failure %q should name the temp file %s", msg, filepath.Base(left[0]))
	}
	if got, _ := os.ReadFile(filepath.Join(dest, "-p", "a.jsonl")); string(got) != "mine" {
		t.Errorf("the copy should be in place, got %q", got)
	}
}

func TestRunReportsATempFileItCannotRemoveAfterAFailedCopy(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")
	opts := withFaults(faulty{
		openFile: readOnlyAfterCreate(isTemp),
		remove:   cannotRemove(isTemp, nil),
	})

	p := only(t, mustRun(t, profiles, dest, opts))
	if p.Copied != 0 || len(p.Failed) != 1 {
		t.Fatalf("got %+v, want the file to fail", p)
	}
	msg := p.Failed[0].Error
	if !strings.Contains(msg, badWriteText) || !strings.Contains(msg, "could not remove the temp file") {
		t.Errorf("failure %q should carry both the copy error and the leftover temp file", msg)
	}
}

// the fallback writes under the real name, so a partial file there is the worst
// leftover, and the user has to know to delete it by hand.
func TestRunReportsAPartialCopyItCannotRemove(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")
	notTemp := func(name string) bool { return !isTemp(name) }
	opts := withFaults(faulty{
		link:     noLinks,
		openFile: readOnlyAfterCreate(notTemp),
		remove:   cannotRemove(notTemp, nil),
	})

	p := only(t, mustRun(t, profiles, dest, opts))
	if p.Copied != 0 || len(p.Failed) != 1 {
		t.Fatalf("got %+v, want the file to fail", p)
	}
	msg := p.Failed[0].Error
	for _, want := range []string{badWriteText, "could not remove the partial copy", "delete it by hand", filepath.Join("-p", "a.jsonl")} {
		if !strings.Contains(msg, want) {
			t.Errorf("failure %q lacks %q", msg, want)
		}
	}
}

func TestRunLostRaceStillReportsATempFileItCannotRemove(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")
	opts := withFaults(faulty{
		link: func(s store, oldname, newname string) error {
			if err := os.WriteFile(filepath.Join(dest, newname), []byte("theirs"), 0o600); err != nil {
				t.Error(err)
			}
			return s.Link(oldname, newname)
		},
		remove: cannotRemove(isTemp, nil),
	})

	p := only(t, mustRun(t, profiles, dest, opts))
	if len(p.Conflicts) != 1 || p.Conflicts[0].Reason != ReasonDiffers {
		t.Errorf("got %+v, want the race judged as a conflict", p)
	}
	if len(p.Failed) != 1 || !strings.Contains(p.Failed[0].Error, "could not remove the temp file") {
		t.Errorf("got %+v, want the leftover temp file reported too", p.Failed)
	}
}
