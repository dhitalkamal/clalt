package reviews

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// fakeInfo is a hand-written os.FileInfo with just a mode.
type fakeInfo struct{ mode fs.FileMode }

func (f fakeInfo) Name() string       { return "x" }
func (f fakeInfo) Size() int64        { return 1 }
func (f fakeInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeInfo) Sys() any           { return nil }

// fakeFS answers Stat from a path->mode map and records every path asked.
type fakeFS struct {
	modes map[string]fs.FileMode
	asked []string
}

func (f *fakeFS) stat(path string) (os.FileInfo, error) {
	f.asked = append(f.asked, path)
	m, ok := f.modes[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return fakeInfo{m}, nil
}

// notOnPath is a LookPath that never finds anything, like a bare launchd PATH.
func notOnPath(name string) (string, error) {
	return "", fmt.Errorf("lookup %s: %w", name, exec.ErrNotFound)
}

func newResolver(modes map[string]fs.FileMode) (Resolver, *fakeFS) {
	f := &fakeFS{modes: modes}
	return Resolver{LookPath: notOnPath, Stat: f.stat, Home: "/home/u"}, f
}

func TestResolvePathLookupWinsOverFallbackDirs(t *testing.T) {
	r, f := newResolver(map[string]fs.FileMode{"/opt/homebrew/bin/claude": 0o755})
	r.LookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	got, err := r.Resolve("claude")
	if err != nil || got != "/usr/bin/claude" {
		t.Fatalf("got %q, %v", got, err)
	}
	if len(f.asked) != 0 {
		t.Errorf("stat ran on %v, want none", f.asked)
	}
}

func TestResolveFallsBackToTheFirstDirThatHasIt(t *testing.T) {
	r, _ := newResolver(map[string]fs.FileMode{
		"/usr/local/bin/claude":     0o755,
		"/home/u/.local/bin/claude": 0o755,
	})
	got, err := r.Resolve("claude")
	if err != nil || got != "/usr/local/bin/claude" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestResolveFallbackOrderAcrossAllFourDirs(t *testing.T) {
	all := []string{"/opt/homebrew/bin/gh", "/usr/local/bin/gh", "/home/u/.local/bin/gh", "/home/u/go/bin/gh"}
	modes := map[string]fs.FileMode{}
	for _, p := range all {
		modes[p] = 0o755
	}
	for i, want := range all {
		r, _ := newResolver(modes)
		got, err := r.Resolve("gh")
		if err != nil || got != want {
			t.Fatalf("step %d: got %q, %v, want %q", i, got, err, want)
		}
		delete(modes, want)
	}
	r, _ := newResolver(modes)
	if _, err := r.Resolve("gh"); err == nil {
		t.Fatal("nothing left, want an error")
	}
}

func TestResolveSkipsDirectoriesAndNonExecutableFiles(t *testing.T) {
	r, _ := newResolver(map[string]fs.FileMode{
		"/opt/homebrew/bin/claude":  fs.ModeDir | 0o755,
		"/usr/local/bin/claude":     0o644,
		"/home/u/.local/bin/claude": 0o700,
	})
	got, err := r.Resolve("claude")
	if err != nil || got != "/home/u/.local/bin/claude" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestResolveAbsolutePathIsReturnedUntouchedEvenWhenMissing(t *testing.T) {
	r, f := newResolver(nil)
	r.LookPath = func(string) (string, error) { t.Error("lookpath ran"); return "", nil }
	got, err := r.Resolve("/abs/claude")
	if err != nil || got != "/abs/claude" {
		t.Fatalf("got %q, %v", got, err)
	}
	if len(f.asked) != 0 {
		t.Errorf("stat ran on %v, want none", f.asked)
	}
}

func TestResolveErrorNamesTheToolAndWrapsTheLookPathError(t *testing.T) {
	r, _ := newResolver(nil)
	_, err := r.Resolve("claude")
	if err == nil {
		t.Fatal("want an error")
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap exec.ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "claude") || !strings.Contains(err.Error(), "/opt/homebrew/bin") {
		t.Errorf("err = %q, want the tool name and the dirs", err)
	}
}

func TestResolveDirsListsTheFourFallbacksInOrder(t *testing.T) {
	r, _ := newResolver(nil)
	want := []string{"/opt/homebrew/bin", "/usr/local/bin", "/home/u/.local/bin", "/home/u/go/bin"}
	got := r.Dirs()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("dirs = %v, want %v", got, want)
	}
}

func TestResolveWithEmptyHomeSkipsTheHomeDirs(t *testing.T) {
	r, f := newResolver(map[string]fs.FileMode{
		"/.local/bin/claude": 0o755,
		"/go/bin/claude":     0o755,
	})
	r.Home = ""
	if _, err := r.Resolve("claude"); err == nil {
		t.Fatal("want an error, home dirs must be skipped")
	}
	for _, p := range f.asked {
		if !strings.HasPrefix(p, "/opt/homebrew/bin/") && !strings.HasPrefix(p, "/usr/local/bin/") {
			t.Errorf("stat ran on %q", p)
		}
	}
}
