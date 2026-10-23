//go:build !windows

package migrate

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

// opening a fifo for reading blocks until a writer shows up, so reaching for
// one would hang the whole command. it has to be skipped untouched.
func TestRunSkipsSpecialFilesWithoutOpeningThem(t *testing.T) {
	profiles, dest := dirs(t)
	root := filepath.Join(profiles, "work", "projects")
	put(t, root, "-p/real.jsonl", "keep")
	if err := syscall.Mkfifo(filepath.Join(root, "-p", "pipe.jsonl"), 0o600); err != nil {
		t.Skipf("no fifo here: %v", err)
	}

	type result struct {
		rep Report
		err error
	}
	done := make(chan result, 1)
	go func() {
		rep, err := Run(profiles, dest, Options{})
		done <- result{rep, err}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatal(r.err)
		}
		p := only(t, r.rep)
		if p.Copied != 1 || p.Skipped != 1 {
			t.Errorf("got %d copied and %d skipped, want 1 and 1", p.Copied, p.Skipped)
		}
		if want := []Skip{{Path: "-p/pipe.jsonl", Kind: "pipe"}}; !reflect.DeepEqual(p.SkippedEntries, want) {
			t.Errorf("skipped entries = %+v, want %+v", p.SkippedEntries, want)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the run hung, it must have opened the fifo")
	}
}

// /dev/null takes every write and refuses to sync, which is the one way to make
// the sync of a temp file fail without a broken disk.
func TestRunReportsAFailedSync(t *testing.T) {
	profiles, dest := dirs(t)
	put(t, profiles, "work/projects/-p/a.jsonl", "mine")
	opts := withFaults(faulty{openFile: func(s store, name string, flag int, perm fs.FileMode) (*os.File, error) {
		if isTemp(name) {
			return os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		}
		return s.OpenFile(name, flag, perm)
	}})

	p := only(t, mustRun(t, profiles, dest, opts))
	if p.Copied != 0 || len(p.Failed) != 1 || !strings.Contains(p.Failed[0].Error, "sync") {
		t.Errorf("got %+v, want a.jsonl failed at the sync", p)
	}
	if _, err := os.Lstat(filepath.Join(dest, "-p", "a.jsonl")); err == nil {
		t.Error("nothing may be left under the final name")
	}
}
