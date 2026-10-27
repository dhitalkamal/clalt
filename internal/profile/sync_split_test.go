package profile

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func isRealDir(t *testing.T, path string) bool {
	t.Helper()
	fi, err := os.Lstat(path)
	return err == nil && fi.IsDir()
}

func TestSyncMakesRealFoldersWithLinksInside(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	before := listTree(t, shared)

	rep := mustSync(t, acct, shared)

	for _, f := range []string{"skills", "plugins"} {
		if !isRealDir(t, filepath.Join(acct, f)) {
			t.Errorf("%s is not a real folder", f)
		}
	}
	for _, n := range append(append([]string{}, skillsLinked...), pluginsLinked...) {
		target, err := os.Readlink(filepath.Join(acct, filepath.FromSlash(n)))
		if err != nil || target != filepath.Join(shared, filepath.FromSlash(n)) {
			t.Errorf("%s -> %q (%v)", n, target, err)
		}
	}
	for _, n := range []string{"skills/synced", "skills/.trash", "plugins/synced", "plugins/known_marketplaces_claudeai.json"} {
		if present(filepath.Join(acct, filepath.FromSlash(n))) {
			t.Errorf("%s was linked, it stays per account", n)
		}
	}
	// what claude.ai downloads goes into the account's own folder
	writeT(t, filepath.Join(acct, "skills", "synced", "a", "SKILL.md"), "downloaded")
	if after := listTree(t, shared); !reflect.DeepEqual(before, after) {
		t.Errorf("the shared home changed:\nbefore %v\nafter  %v", before, after)
	}
	if !rep.Complete() || rep.Err() != nil {
		t.Errorf("report = %+v", rep)
	}
}

func TestSyncReplacesAWholeFolderLinkWithoutTouchingTheSharedFolder(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	mustSymlink(t, filepath.Join(shared, "skills"), filepath.Join(acct, "skills"))
	before := listTree(t, shared)

	rep := mustSync(t, acct, shared)

	if !isRealDir(t, filepath.Join(acct, "skills")) {
		t.Fatal("skills is still a link")
	}
	if after := listTree(t, shared); !reflect.DeepEqual(before, after) {
		t.Errorf("the shared home changed:\nbefore %v\nafter  %v", before, after)
	}
	if target, _ := os.Readlink(filepath.Join(acct, "skills", "review")); target != filepath.Join(shared, "skills", "review") {
		t.Errorf("skills/review -> %q", target)
	}
	var folder *Step
	for i := range rep.Repointed {
		if rep.Repointed[i].Name == "skills" {
			folder = &rep.Repointed[i]
		}
	}
	if folder == nil || !folder.Folder || folder.Old != filepath.Join(shared, "skills") {
		t.Errorf("Repointed = %+v, want the skills folder step with its old target", rep.Repointed)
	}
}

func TestApplyNeverWritesThroughALinkedFolder(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	mustSymlink(t, filepath.Join(shared, "skills"), filepath.Join(acct, "skills"))
	before := listTree(t, shared)

	for _, st := range []Step{
		{Name: "skills/new", Action: Link, Source: filepath.Join(shared, "commands")},
		{Name: "skills/review", Action: Repoint, Source: filepath.Join(shared, "commands")},
		{Name: "skills/review", Action: Remove},
	} {
		if _, err := apply(acct, st); err == nil {
			t.Errorf("apply(%+v) went through the link", st)
		}
	}
	if after := listTree(t, shared); !reflect.DeepEqual(before, after) {
		t.Errorf("the shared home changed:\nbefore %v\nafter  %v", before, after)
	}
}

func TestSyncSkipsTheEntriesOfAFolderItCouldNotReplace(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission bits are not enforced here")
	}
	root, shared := sharedFixture(t)
	mkdirT(t, filepath.Join(shared, "skills", "review"))
	acct := filepath.Join(root, "acct")
	linkAll(t, acct, shared)
	mustSymlink(t, filepath.Join(shared, "skills"), filepath.Join(acct, "skills"))
	// the lock file exists, so taking the lock needs no write access
	writeT(t, filepath.Join(acct, syncLockName), "")
	before := listTree(t, shared)
	if err := os.Chmod(acct, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(acct, 0o755) })

	rep := mustSync(t, acct, shared)

	if len(rep.Errors) != 1 || rep.Errors[0].Name != "skills" {
		t.Errorf("Errors = %v, want exactly one, for skills", rep.Errors)
	}
	if after := listTree(t, shared); !reflect.DeepEqual(before, after) {
		t.Errorf("the shared home changed:\nbefore %v\nafter  %v", before, after)
	}
}

func TestSyncWithSplitFoldersIsQuietTheSecondTime(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	mustSync(t, acct, shared)
	before := listTree(t, acct)

	rep := mustSync(t, acct, shared)

	if !reflect.DeepEqual(rep, Report{}) {
		t.Errorf("second sync reported %+v", rep)
	}
	if after := listTree(t, acct); !reflect.DeepEqual(before, after) {
		t.Errorf("second sync changed the folder:\nbefore %v\nafter  %v", before, after)
	}
}

func TestSyncCleansUpInsideASplitFolderAndKeepsWhatIsTheAccountsOwn(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	linkAll(t, acct, shared)
	linkSplit(t, acct, shared)
	skills := filepath.Join(acct, "skills")
	mustSymlink(t, filepath.Join(shared, "skills", "gone"), filepath.Join(skills, "gone"))
	mustSymlink(t, filepath.Join(shared, "skills", "synced"), filepath.Join(skills, "synced"))
	if err := os.Remove(filepath.Join(skills, "review")); err != nil {
		t.Fatal(err)
	}
	writeT(t, filepath.Join(skills, "review", "mine.md"), "own")
	writeT(t, filepath.Join(skills, "mine", "x.md"), "own")

	rep := mustSync(t, acct, shared)

	if !reflect.DeepEqual(rep.Removed, []string{"skills/gone", "skills/synced"}) {
		t.Errorf("Removed = %v", rep.Removed)
	}
	if names := stepNames(rep.Kept); !reflect.DeepEqual(names, []string{"skills/review"}) {
		t.Errorf("Kept = %v", names)
	}
	for _, f := range []string{"review/mine.md", "mine/x.md"} {
		if got := readT(t, filepath.Join(skills, filepath.FromSlash(f))); got != "own" {
			t.Errorf("%s now holds %q", f, got)
		}
	}
	if rep.Complete() {
		t.Error("a kept entry means the account is not complete")
	}
}

func TestApplyCreatesTheFolderForTheFirstEntryAndRefusesAFile(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	mkdirT(t, acct)

	done, err := apply(acct, Step{Name: "skills/review", Action: Link, Source: filepath.Join(shared, "skills", "review")})
	if err != nil || !done || !isRealDir(t, filepath.Join(acct, "skills")) {
		t.Fatalf("apply = (%v, %v), skills real folder: %v", done, err, isRealDir(t, filepath.Join(acct, "skills")))
	}

	writeT(t, filepath.Join(acct, "plugins"), "a file")
	_, err = apply(acct, Step{Name: "plugins/cache", Action: Link, Source: filepath.Join(shared, "plugins", "cache")})
	if err == nil {
		t.Error("apply linked inside a file")
	}
	if got := readT(t, filepath.Join(acct, "plugins")); got != "a file" {
		t.Errorf("the file now holds %q", got)
	}
}

func TestApplyToleratesTheFolderAppearingFirst(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	mkdirT(t, acct)
	old := mkdir
	mkdir = func(path string, perm os.FileMode) error {
		// claude made the folder between our look and our mkdir
		if err := os.Mkdir(path, perm); err != nil {
			t.Error(err)
		}
		return os.Mkdir(path, perm)
	}
	t.Cleanup(func() { mkdir = old })

	done, err := apply(acct, Step{Name: "skills/review", Action: Link, Source: filepath.Join(shared, "skills", "review")})

	if err != nil || !done {
		t.Fatalf("apply = (%v, %v)", done, err)
	}
	if target, _ := os.Readlink(filepath.Join(acct, "skills", "review")); target != filepath.Join(shared, "skills", "review") {
		t.Errorf("skills/review -> %q", target)
	}
}

func TestApplyDoesNotWriteThroughALinkThatTookTheFoldersName(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "acct")
	mkdirT(t, acct)
	before := listTree(t, shared)
	old := mkdir
	mkdir = func(path string, perm os.FileMode) error {
		mustSymlink(t, filepath.Join(shared, "skills"), path)
		return os.Mkdir(path, perm)
	}
	t.Cleanup(func() { mkdir = old })

	if _, err := apply(acct, Step{Name: "skills/new", Action: Link, Source: filepath.Join(shared, "commands")}); err == nil {
		t.Error("apply wrote through a link that appeared in the folder's place")
	}
	if after := listTree(t, shared); !reflect.DeepEqual(before, after) {
		t.Errorf("the shared home changed:\nbefore %v\nafter  %v", before, after)
	}
}
