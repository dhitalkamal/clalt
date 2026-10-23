package profile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// these pin the checks apply makes again right before it changes anything: the
// plan is older than the apply, and claude may have written in between.

func TestReplaceWithFolderLeavesARealFileAlone(t *testing.T) {
	acct := t.TempDir()
	path := filepath.Join(acct, "skills")
	writeT(t, path, "took the place of the link")

	done, err := apply(acct, Step{Name: "skills", Action: Repoint, Folder: true})

	if done || err == nil || !strings.Contains(err.Error(), "not a link") {
		t.Errorf("apply = (%v, %v), want an error saying it is not a link any more", done, err)
	}
	if got := readT(t, path); got != "took the place of the link" {
		t.Errorf("the file now holds %q", got)
	}
}

func TestReplaceWithFolderLeavesAFolderWithContentAlone(t *testing.T) {
	acct := t.TempDir()
	writeT(t, filepath.Join(acct, "skills", "mine", "SKILL.md"), "own")

	done, err := apply(acct, Step{Name: "skills", Action: Repoint, Folder: true})

	if done || err != nil {
		t.Errorf("apply = (%v, %v), want nothing done and no error", done, err)
	}
	if got := readT(t, filepath.Join(acct, "skills", "mine", "SKILL.md")); got != "own" {
		t.Errorf("the folder content is now %q", got)
	}
}

func TestReplaceWithFolderMakesTheFolderWhenTheLinkIsAlreadyGone(t *testing.T) {
	acct := t.TempDir()

	done, err := apply(acct, Step{Name: "skills", Action: Repoint, Folder: true})

	if !done || err != nil || !isRealDir(t, filepath.Join(acct, "skills")) {
		t.Errorf("apply = (%v, %v), skills a real folder: %v", done, err, isRealDir(t, filepath.Join(acct, "skills")))
	}
}

func TestReplaceWithFolderAcceptsAFolderThatAppearedFirst(t *testing.T) {
	acct := t.TempDir()
	mustSymlink(t, t.TempDir(), filepath.Join(acct, "skills"))
	old := mkdir
	mkdir = func(path string, perm os.FileMode) error {
		// claude made the folder right after the link went
		if err := os.Mkdir(path, perm); err != nil {
			t.Error(err)
		}
		return os.Mkdir(path, perm)
	}
	t.Cleanup(func() { mkdir = old })

	done, err := apply(acct, Step{Name: "skills", Action: Repoint, Folder: true})

	if !done || err != nil || !isRealDir(t, filepath.Join(acct, "skills")) {
		t.Errorf("apply = (%v, %v)", done, err)
	}
}

func TestReplaceWithFolderReportsAFailureToMakeTheFolder(t *testing.T) {
	acct := t.TempDir()
	mustSymlink(t, t.TempDir(), filepath.Join(acct, "skills"))
	boom := errors.New("disk full")
	old := mkdir
	mkdir = func(string, os.FileMode) error { return boom }
	t.Cleanup(func() { mkdir = old })

	done, err := apply(acct, Step{Name: "skills", Action: Repoint, Folder: true})

	if done || !errors.Is(err, boom) {
		t.Errorf("apply = (%v, %v), want the failure", done, err)
	}
}

func TestApplyMakesTheFolderOnlyForALink(t *testing.T) {
	_, shared := sharedFixture(t)
	splitShared(t, shared)
	for _, st := range []Step{
		{Name: "skills/review", Action: Remove},
		{Name: "skills/review", Action: Repoint, Source: filepath.Join(shared, "skills", "review")},
	} {
		acct := t.TempDir()
		apply(acct, st)
		if present(filepath.Join(acct, "skills")) {
			t.Errorf("apply of a %v made the folder", st.Action)
		}
	}
}

func TestTempLinkSkipsNamesThatAreTaken(t *testing.T) {
	dir := t.TempDir()
	mustSymlink(t, dir, filepath.Join(dir, "probe"))
	path := filepath.Join(dir, "review")
	taken := func(i int) string {
		return filepath.Join(dir, fmt.Sprintf(".review.clalt-%d-%d", os.Getpid(), i))
	}
	writeT(t, taken(0), "a leftover")

	tmp, err := tempLink("/shared/review", path)

	if err != nil || tmp != taken(1) {
		t.Fatalf("tempLink = %q, %v, want %q", tmp, err, taken(1))
	}
	if got := readT(t, taken(0)); got != "a leftover" {
		t.Errorf("the taken name now holds %q", got)
	}
	// with every name taken it gives up instead of replacing one
	for i := 2; i < 16; i++ {
		writeT(t, taken(i), "taken")
	}
	if tmp, err := tempLink("/shared/review", path); err == nil {
		t.Errorf("tempLink = %q, want an error when all names are taken", tmp)
	}
}
