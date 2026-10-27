package profile

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func mustSync(t *testing.T, profileDir, shared string) Report {
	t.Helper()
	rep, err := Sync(profileDir, shared)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	return rep
}

func TestSyncCreatesTheFolderAndLinksAbsolute(t *testing.T) {
	root, shared := sharedFixture(t)
	profileDir := filepath.Join(root, "accounts", "robin")

	rep := mustSync(t, profileDir, shared)

	if !reflect.DeepEqual(rep.Linked, shareableNames) {
		t.Errorf("Linked = %v, want %v", rep.Linked, shareableNames)
	}
	if len(rep.Repointed)+len(rep.Removed)+len(rep.Kept)+len(rep.Errors) != 0 {
		t.Errorf("unexpected report: %+v", rep)
	}
	for _, n := range shareableNames {
		target, err := os.Readlink(filepath.Join(profileDir, n))
		if err != nil || target != filepath.Join(shared, n) {
			t.Errorf("%s -> %q (%v), want %q", n, target, err, filepath.Join(shared, n))
		}
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(profileDir)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o700 {
			t.Errorf("profile folder mode = %v, want 0700", fi.Mode().Perm())
		}
	}
}

func TestSyncIsIdempotent(t *testing.T) {
	root, shared := sharedFixture(t)
	profileDir := filepath.Join(root, "acct")
	mustSync(t, profileDir, shared)
	before := listTree(t, profileDir)

	rep := mustSync(t, profileDir, shared)

	if !reflect.DeepEqual(rep, Report{}) {
		t.Errorf("second sync reported %+v", rep)
	}
	if after := listTree(t, profileDir); !reflect.DeepEqual(before, after) {
		t.Errorf("second sync changed the folder:\nbefore %v\nafter  %v", before, after)
	}
}

func TestSyncRepointsRemovesAndKeeps(t *testing.T) {
	root, shared := sharedFixture(t)
	profileDir := filepath.Join(root, "acct")
	linkAll(t, profileDir, shared)
	other := filepath.Join(root, "other")
	mkdirT(t, other)
	// aimed elsewhere
	if err := os.Remove(filepath.Join(profileDir, "commands")); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, other, filepath.Join(profileDir, "commands"))
	// old links: one dangling, one for a name that must not be shared
	mustSymlink(t, filepath.Join(shared, "hooks"), filepath.Join(profileDir, "hooks"))
	mustSymlink(t, filepath.Join(shared, "sessions"), filepath.Join(profileDir, "sessions"))
	// a real file where a link belongs
	if err := os.Remove(filepath.Join(profileDir, "settings.json")); err != nil {
		t.Fatal(err)
	}
	writeT(t, filepath.Join(profileDir, "settings.json"), "my own")

	rep := mustSync(t, profileDir, shared)

	if !reflect.DeepEqual(stepNames(rep.Repointed), []string{"commands"}) ||
		!reflect.DeepEqual(rep.Removed, []string{"hooks", "sessions"}) ||
		!reflect.DeepEqual(stepNames(rep.Kept), []string{"settings.json"}) ||
		len(rep.Linked)+len(rep.Errors) != 0 {
		t.Errorf("report = %+v", rep)
	}
	// repointed and kept names carry what a person needs to understand them
	if got := rep.Repointed[0]; got.Old != other || got.Source != filepath.Join(shared, "commands") || got.Why == "" {
		t.Errorf("repointed step = %+v", got)
	}
	if got := rep.Kept[0]; got.Why == "" || got.Source != filepath.Join(shared, "settings.json") {
		t.Errorf("kept step = %+v", got)
	}
	if target, _ := os.Readlink(filepath.Join(profileDir, "commands")); target != filepath.Join(shared, "commands") {
		t.Errorf("commands -> %q after repoint", target)
	}
	if present(filepath.Join(profileDir, "hooks")) || present(filepath.Join(profileDir, "sessions")) {
		t.Error("stale links were not removed")
	}
	if got := readT(t, filepath.Join(profileDir, "settings.json")); got != "my own" {
		t.Errorf("kept file now holds %q", got)
	}
	// the shared home is untouched
	if !present(filepath.Join(shared, "sessions")) {
		t.Error("removing the link removed the shared folder")
	}
}

func TestSyncNeverCopiesInPlaceOfALink(t *testing.T) {
	root, shared := sharedFixture(t)
	profileDir := filepath.Join(root, "acct")
	boom := errors.New("A required privilege is not held by the client.")
	old := symlink
	symlink = func(string, string) error { return boom }
	t.Cleanup(func() { symlink = old })

	rep := mustSync(t, profileDir, shared)

	if len(rep.Linked) != 0 {
		t.Errorf("Linked = %v with every symlink failing", rep.Linked)
	}
	var names []string
	for _, e := range rep.Errors {
		names = append(names, e.Name)
		if !errors.Is(e, boom) {
			t.Errorf("%s: error %v does not wrap the symlink failure", e.Name, e)
		}
	}
	if !reflect.DeepEqual(names, shareableNames) {
		t.Errorf("errors for %v, want %v", names, shareableNames)
	}
	for _, e := range mustReadDir(t, profileDir) {
		if e.Name() != syncLockName {
			t.Errorf("profile folder holds %q, want nothing but the lock file (no copies)", e.Name())
		}
	}
}

func TestSyncKeepsGoingAfterOneItemFails(t *testing.T) {
	root, shared := sharedFixture(t)
	profileDir := filepath.Join(root, "acct")
	old := symlink
	symlink = func(src, dst string) error {
		if filepath.Base(dst) == "projects" {
			return errors.New("denied")
		}
		return old(src, dst)
	}
	t.Cleanup(func() { symlink = old })

	rep := mustSync(t, profileDir, shared)

	if len(rep.Errors) != 1 || rep.Errors[0].Name != "projects" {
		t.Errorf("Errors = %v, want one for projects", rep.Errors)
	}
	if len(rep.Linked) != len(shareableNames)-1 {
		t.Errorf("Linked = %v, want the other %d names", rep.Linked, len(shareableNames)-1)
	}
}

func TestSyncRefusesTheSharedHomeAsProfileFolder(t *testing.T) {
	_, shared := sharedFixture(t)
	before := listTree(t, shared)
	if _, err := Sync(shared, shared); err == nil {
		t.Fatal("Sync accepted the shared home as the profile folder")
	}
	if after := listTree(t, shared); !reflect.DeepEqual(before, after) {
		t.Errorf("Sync changed the shared home:\nbefore %v\nafter  %v", before, after)
	}
}

func TestApplyNeverRemovesARealFileOrFolder(t *testing.T) {
	root := t.TempDir()
	writeT(t, filepath.Join(root, "file"), "keep me")
	writeT(t, filepath.Join(root, "dir", "inner"), "keep me too")
	for _, n := range []string{"file", "dir"} {
		for _, a := range []Action{Remove, Repoint} {
			if _, err := apply(root, Step{Name: n, Action: a, Source: "/x"}); err == nil {
				t.Errorf("apply(%v, %s) removed or accepted a real entry", a, n)
			}
		}
	}
	if readT(t, filepath.Join(root, "file")) != "keep me" || readT(t, filepath.Join(root, "dir", "inner")) != "keep me too" {
		t.Error("real content was damaged")
	}
}

func TestApplyToleratesAnotherSyncGettingThereFirst(t *testing.T) {
	root, shared := sharedFixture(t)
	profileDir := filepath.Join(root, "acct")
	mustSymlink(t, filepath.Join(shared, "commands"), filepath.Join(profileDir, "commands"))

	// the plan said Link, but the same link appeared in the meantime
	done, err := apply(profileDir, Step{Name: "commands", Action: Link, Source: filepath.Join(shared, "commands")})
	if err != nil || done {
		t.Errorf("apply = (%v, %v), want (false, nil)", done, err)
	}
	// and a link someone else removed first is not an error either
	done, err = apply(profileDir, Step{Name: "agents", Action: Remove})
	if err != nil || done {
		t.Errorf("apply remove of a missing name = (%v, %v), want (false, nil)", done, err)
	}
	// a different link in the way is still an error
	mustSymlink(t, filepath.Join(shared, "projects"), filepath.Join(profileDir, "agents"))
	if _, err := apply(profileDir, Step{Name: "agents", Action: Link, Source: filepath.Join(shared, "agents")}); err == nil {
		t.Error("apply replaced or accepted a link that aims elsewhere")
	}
}
