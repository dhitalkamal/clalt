package profile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRepointFallsBackWhereARenameCannotReplaceAFolderLink(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	wrongLink(t, root, shared, acct)
	oldGOOS, oldRename := goos, rename
	goos = "windows"
	// MoveFileEx cannot replace a link to a folder
	rename = func(string, string) error { return errors.New("access is denied") }
	t.Cleanup(func() { goos, rename = oldGOOS, oldRename })

	done, err := apply(acct, repointStep(shared))

	if !done || err != nil {
		t.Fatalf("apply = (%v, %v)", done, err)
	}
	if target, _ := os.Readlink(filepath.Join(acct, "commands")); target != filepath.Join(shared, "commands") {
		t.Errorf("commands -> %q", target)
	}
	if left := tempLinks(t, acct); len(left) != 0 {
		t.Errorf("temporary links left behind: %v", left)
	}
}

func TestRepointDoesNotRemoveTheLinkFirstOutsideWindows(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	other := wrongLink(t, root, shared, acct)
	boom := errors.New("boom")
	oldGOOS, oldRename := goos, rename
	goos = "linux"
	rename = func(string, string) error { return boom }
	t.Cleanup(func() { goos, rename = oldGOOS, oldRename })

	if _, err := apply(acct, repointStep(shared)); !errors.Is(err, boom) {
		t.Errorf("apply err = %v, want the rename error", err)
	}
	if target, err := os.Readlink(filepath.Join(acct, "commands")); err != nil || target != other {
		t.Errorf("commands -> %q (%v), want the old link kept at %q", target, err, other)
	}
	if left := tempLinks(t, acct); len(left) != 0 {
		t.Errorf("temporary links left behind: %v", left)
	}
}

func TestRepointFallbackReportsALinkItCannotMake(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	wrongLink(t, root, shared, acct)
	boom := errors.New("no privilege")
	oldGOOS, oldRename, oldSymlink := goos, rename, symlink
	goos = "windows"
	rename = func(string, string) error { return errors.New("access is denied") }
	t.Cleanup(func() { goos, rename, symlink = oldGOOS, oldRename, oldSymlink })
	calls := 0
	symlink = func(source, path string) error {
		// the temporary link works, the one made after the removal does not
		if calls++; calls > 1 {
			return boom
		}
		return oldSymlink(source, path)
	}

	done, err := apply(acct, repointStep(shared))

	if done || !errors.Is(err, boom) {
		t.Errorf("apply = (%v, %v), want the link error", done, err)
	}
	if left := tempLinks(t, acct); len(left) != 0 {
		t.Errorf("temporary links left behind: %v", left)
	}
}
