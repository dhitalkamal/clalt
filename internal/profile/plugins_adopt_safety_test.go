package profile

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAdoptLeavesASharedSymlinkAlone(t *testing.T) {
	root, shared, acct := adoptFixture(t)
	target := filepath.Join(root, "elsewhere.json")
	writeT(t, target, "target content")
	sharedFile := filepath.Join(shared, "plugins", "known_marketplaces.json")
	if err := os.Remove(sharedFile); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, target, sharedFile)
	writeT(t, filepath.Join(acct, "plugins", "known_marketplaces.json"), "mine")

	rep := mustSync(t, acct, shared)

	if got, _ := os.Readlink(sharedFile); got != target || readT(t, target) != "target content" {
		t.Errorf("shared link or target changed: %q", got)
	}
	if len(rep.Adopted) != 0 || len(rep.Kept) == 0 || len(backupsOf(t, root)) != 0 {
		t.Errorf("report = %+v", rep)
	}
}

func TestAdoptSkipsAnOddLayoutAndWritesNoBackup(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	acct := filepath.Join(root, "somewhere", "work")
	accFile := filepath.Join(acct, "plugins", "known_marketplaces.json")
	writeT(t, accFile, "mine")
	before := readT(t, filepath.Join(shared, "plugins", "known_marketplaces.json"))

	rep := mustSync(t, acct, shared)

	if len(rep.Adopted) != 0 || len(rep.Kept) == 0 {
		t.Errorf("report = %+v", rep)
	}
	if readT(t, accFile) != "mine" || readT(t, filepath.Join(shared, "plugins", "known_marketplaces.json")) != before {
		t.Error("something was written")
	}
	if present(filepath.Join(root, "backups")) || present(filepath.Join(root, "somewhere", "backups")) {
		t.Error("a backup folder was made")
	}
}

func TestAdoptsOfTwoAccountsAtOnceLoseNothing(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	sharedFile := filepath.Join(shared, "plugins", "known_marketplaces.json")
	setAge(t, sharedFile, time.Hour)
	var accts []string
	for _, id := range []string{"a", "b"} {
		acct := filepath.Join(root, "accounts", id)
		writeT(t, filepath.Join(acct, "plugins", "known_marketplaces.json"), "content "+id)
		accts = append(accts, acct)
	}
	var wg sync.WaitGroup
	for _, a := range accts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := Sync(a, shared); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	seen := readT(t, sharedFile)
	var all []string
	filepath.WalkDir(filepath.Join(root, "backups"), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			all = append(all, readT(t, p))
		}
		return nil
	})
	all = append(all, seen)
	joined := strings.Join(all, "|")
	for _, want := range []string{"content a", "content b", "x"} {
		if !strings.Contains(joined, want) {
			t.Errorf("%q is lost: shared %q, backups %v", want, seen, all)
		}
	}
}
