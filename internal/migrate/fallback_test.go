package migrate

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// only "this filesystem has no hard links" may drop to the in-place write. A
// full disk, an i/o error or a permission problem has to come back as that
// error, not be retried non-atomically under the final name.
func TestRunFallsBackOnlyWhenLinksAreUnsupported(t *testing.T) {
	for name, c := range map[string]struct {
		err      error
		wantCopy bool
	}{
		"unsupported": {errors.ErrUnsupported, true},
		"EPERM":       {syscall.EPERM, true},
		"EIO":         {syscall.EIO, false},
		"ENOSPC":      {syscall.ENOSPC, false},
		"EACCES":      {syscall.EACCES, false},
	} {
		t.Run(name, func(t *testing.T) {
			profiles, dest := dirs(t)
			put(t, profiles, "work/projects/-p/a.jsonl", "mine")
			opts := withFaults(faulty{link: func(s store, oldname, newname string) error {
				return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: c.err}
			}})

			p := only(t, mustRun(t, profiles, dest, opts))
			if c.wantCopy {
				if p.Copied != 1 || p.InPlace != 1 || len(p.Failed) != 0 {
					t.Errorf("got %+v, want the in-place fallback to copy it", p)
				}
				return
			}
			if p.Copied != 0 || len(p.Failed) != 1 || !strings.Contains(p.Failed[0].Error, "link") {
				t.Errorf("got %+v, want the link error reported", p)
			}
			if _, err := os.Lstat(filepath.Join(dest, "-p", "a.jsonl")); err == nil {
				t.Error("nothing may appear under the final name for a real link failure")
			}
			noTempFiles(t, dest)
		})
	}
}

// the source may have grown since the temp copy was made, and what lands under
// the final name has to be what the report counted.
func TestRunFallbackWritesWhatTheTempHoldsNotTheGrownSource(t *testing.T) {
	profiles, dest := dirs(t)
	src := put(t, profiles, "work/projects/-p/a.jsonl", "0123456789")
	opts := withFaults(faulty{link: func(s store, oldname, newname string) error {
		f, err := os.OpenFile(src, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Error(err)
		} else {
			_, _ = f.WriteString("-grew")
			f.Close()
		}
		return noLinks(s, oldname, newname)
	}})

	p := only(t, mustRun(t, profiles, dest, opts))
	got, err := os.ReadFile(filepath.Join(dest, "-p", "a.jsonl"))
	if err != nil || string(got) != "0123456789" || p.Bytes != 10 {
		t.Errorf("destination holds %q (%v) and the report says %d bytes, want the 10 bytes the temp held", got, err, p.Bytes)
	}
}

// once the filesystem has said it cannot link, trying again for every file is
// a wasted temp copy each time.
func TestRunFallbackIsRememberedAfterTheFirstFile(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "one")
	put(t, profiles, "work/projects/-p/b.jsonl", "two")
	put(t, profiles, "work/projects/-q/c.jsonl", "three")
	links, temps := 0, 0
	opts := withFaults(faulty{
		link: func(s store, oldname, newname string) error {
			links++
			return noLinks(s, oldname, newname)
		},
		openFile: func(s store, name string, flag int, perm fs.FileMode) (*os.File, error) {
			if strings.HasPrefix(filepath.Base(name), tempPrefix) {
				temps++
			}
			return s.OpenFile(name, flag, perm)
		},
	})

	rep := mustRun(t, profiles, dest, opts)
	p := only(t, rep)
	if links != 1 || temps != 1 {
		t.Errorf("link tried %d times with %d temp files, want once each", links, temps)
	}
	if p.Copied != 3 || p.InPlace != 3 || rep.Total.InPlace != 3 {
		t.Errorf("got %+v and total %+v, want 3 copied and 3 written in place", p, rep.Total)
	}
	for rel, want := range map[string]string{"-p/a.jsonl": "one", "-p/b.jsonl": "two", "-q/c.jsonl": "three"} {
		if got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(rel))); err != nil || string(got) != want {
			t.Errorf("%s = %q (%v), want %q", rel, got, err, want)
		}
	}
	noTempFiles(t, dest)
}

func TestRunNoFileIsCountedInPlaceWhenLinksWork(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "one")

	rep := mustRun(t, profiles, dest, Options{})
	if p := only(t, rep); p.Copied != 1 || p.InPlace != 0 || rep.Total.InPlace != 0 {
		t.Errorf("got %+v and total %+v, want none written in place", p, rep.Total)
	}
}
