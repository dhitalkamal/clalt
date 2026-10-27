package profile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// loopLink makes a symlink that points at itself. Stat on it fails with
// something other than "does not exist", which is the case that must not be
// mistaken for a dangling link.
func loopLink(t *testing.T, path string) string {
	t.Helper()
	mustSymlink(t, filepath.Base(path), path)
	_, err := os.Stat(path)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Skipf("a self-pointing link is not an unreadable entry here (err: %v)", err)
	}
	var pe *fs.PathError
	if !errors.As(err, &pe) {
		t.Fatalf("unexpected error type %T", err)
	}
	return pe.Err.Error()
}

func TestPlanKeepsAnEntryItCannotRead(t *testing.T) {
	root, shared := sharedFixture(t)
	if err := os.RemoveAll(filepath.Join(shared, "agents")); err != nil {
		t.Fatal(err)
	}
	reason := loopLink(t, filepath.Join(shared, "agents"))
	acct := filepath.Join(root, "acct")

	got := stepMap(mustPlan(t, acct, shared))

	st, ok := got["agents"]
	if !ok || st.Action != Keep || st.Source != filepath.Join(shared, "agents") || !strings.Contains(st.Why, reason) {
		t.Errorf("agents = %+v, want a Keep that carries %q", st, reason)
	}
	if _, ok := got["commands"]; !ok {
		t.Error("an unreadable entry stopped the rest from being planned")
	}
}

func TestPlanDoesNotRemoveTheLinkOfAnEntryItCannotRead(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	linkAll(t, acct, shared)
	if err := os.RemoveAll(filepath.Join(shared, "agents")); err != nil {
		t.Fatal(err)
	}
	loopLink(t, filepath.Join(shared, "agents"))

	got := stepMap(mustPlan(t, acct, shared))

	if len(got) != 1 || got["agents"].Action != Keep {
		t.Fatalf("planned %v, want only a Keep for agents", got)
	}
	rep := mustSync(t, acct, shared)
	if len(rep.Removed) != 0 || len(rep.Kept) != 1 {
		t.Errorf("report = %+v", rep)
	}
	if _, err := os.Readlink(filepath.Join(acct, "agents")); err != nil {
		t.Errorf("the agents link was removed: %v", err)
	}
}

func TestPlanIgnoresUnreadableEntriesThatCouldNeverBeLinked(t *testing.T) {
	// whatever these turn out to be, the rules never link them
	never := []string{".credentials.json", ".claude.json", "sessions", "cache"}
	root, shared := sharedFixture(t)
	for _, n := range never {
		if err := os.RemoveAll(filepath.Join(shared, n)); err != nil {
			t.Fatal(err)
		}
		loopLink(t, filepath.Join(shared, n))
	}
	acct := filepath.Join(root, "acct")

	for _, st := range mustPlan(t, acct, shared) {
		for _, n := range never {
			if st.Name == n {
				t.Errorf("planned %+v for an entry that is never linked", st)
			}
		}
	}
}

func TestPlanKeepsASplitFolderItCannotRead(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission bits are not enforced here")
	}
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	mustSync(t, acct, shared)
	locked := filepath.Join(shared, "skills")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	got := stepMap(mustPlan(t, acct, shared))

	st := got["skills"]
	if len(got) != 1 || st.Action != Keep || st.Source != locked || !strings.Contains(st.Why, "permission denied") {
		t.Fatalf("planned %v, want one Keep for skills that carries the permission error", got)
	}
	if rep := mustSync(t, acct, shared); len(rep.Removed) != 0 {
		t.Errorf("Removed = %v while the shared folder cannot be read", rep.Removed)
	}
	for _, n := range skillsLinked {
		if _, err := os.Readlink(filepath.Join(acct, filepath.FromSlash(n))); err != nil {
			t.Errorf("%s lost its link: %v", n, err)
		}
	}
}
