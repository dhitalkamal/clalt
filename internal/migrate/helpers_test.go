package migrate

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// dirs returns a profiles folder and a shared projects path under one temp
// root. neither exists yet.
func dirs(t *testing.T) (profiles, dest string) {
	t.Helper()
	root := t.TempDir()
	return filepath.Join(root, "profiles"), filepath.Join(root, "claude", "projects")
}

func put(t *testing.T, root, rel, content string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// symlink skips the test only where the OS will not create a link (windows
// without developer mode). anywhere else a link that cannot be made is a broken
// setup, and skipping would hide it.
func symlink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlinks not available: %v", err)
		}
		t.Fatalf("symlink: %v", err)
	}
}

func mustRun(t *testing.T, profiles, dest string, opts Options) Report {
	t.Helper()
	rep, err := Run(profiles, dest, opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return rep
}

func only(t *testing.T, rep Report) Profile {
	t.Helper()
	if len(rep.Profiles) != 1 {
		t.Fatalf("got %d profiles, want 1: %+v", len(rep.Profiles), rep.Profiles)
	}
	return rep.Profiles[0]
}

// resolved is the real location of path, so tests agree with the code on
// systems where the temp dir itself sits behind a link (macOS /var).
func resolved(t *testing.T, path string) string {
	t.Helper()
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

// snapshot describes every entry under root without following links: kind,
// permissions, size, content hash and mtime. two snapshots match only when
// nothing under root changed.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			to, _ := os.Readlink(path)
			snap[rel] = "link " + to
		case d.IsDir():
			snap[rel] = "dir " + info.Mode().Perm().String()
		default:
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			snap[rel] = fmt.Sprintf("file %s %d %x %d", info.Mode().Perm(), info.Size(), sha256.Sum256(data), info.ModTime().UnixNano())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return snap
}

// age sets an old mtime on path, so a later change to it shows in a snapshot.
func age(t *testing.T, path string) {
	t.Helper()
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

// lockOut removes every permission from path for the rest of the test. it skips
// where that cannot work: windows has no such bits and root ignores them.
func lockOut(t *testing.T, path string) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission bits do not stop this user")
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o755) })
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
}

// readOnly stops writes into path for the rest of the test, writeOnly stops it
// being opened for reading. both skip where permission bits cannot stop the
// user.
func readOnly(t *testing.T, path string)  { t.Helper(); withMode(t, path, 0o500) }
func writeOnly(t *testing.T, path string) { t.Helper(); withMode(t, path, 0o300) }

func withMode(t *testing.T, path string, mode fs.FileMode) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission bits do not stop this user")
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o755) })
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

// noTempFiles fails when a half-finished copy left its temp file behind.
func noTempFiles(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Lstat(root); errors.Is(err, fs.ErrNotExist) {
		return // no folder, so no temp file in it
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasPrefix(d.Name(), tempPrefix) {
			t.Errorf("temp file left behind: %s", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func sameSnapshot(t *testing.T, want, got map[string]string) {
	t.Helper()
	for k, w := range want {
		g, ok := got[k]
		switch {
		case !ok:
			t.Errorf("%s: removed", k)
		case g != w:
			t.Errorf("%s: changed\n  was %s\n  now %s", k, w, g)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("%s: added", k)
		}
	}
}
