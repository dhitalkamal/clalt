package profile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/fslock"
)

// wrongLink gives acct a commands link that aims at a folder other than the
// shared one, and returns where it aims.
func wrongLink(t *testing.T, root, shared, acct string) string {
	t.Helper()
	linkAll(t, acct, shared)
	other := filepath.Join(root, "other")
	mkdirT(t, other)
	if err := os.Remove(filepath.Join(acct, "commands")); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, other, filepath.Join(acct, "commands"))
	return other
}

func repointStep(shared string) Step {
	return Step{Name: "commands", Action: Repoint, Source: filepath.Join(shared, "commands")}
}

func tempLinks(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".clalt-") && e.Name() != syncLockName {
			out = append(out, e.Name())
		}
	}
	return out
}

func TestRepointNeverLeavesTheNameWithoutALink(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	other := wrongLink(t, root, shared, acct)
	boom := errors.New("boom")

	// the new link cannot be made
	oldSymlink := symlink
	symlink = func(string, string) error { return boom }
	if _, err := apply(acct, repointStep(shared)); !errors.Is(err, boom) {
		t.Errorf("apply err = %v", err)
	}
	symlink = oldSymlink
	// or cannot be moved into place. windows would then fall back to removing the
	// link, which has tests of its own
	oldRename, oldGOOS := rename, goos
	rename, goos = func(string, string) error { return boom }, "linux"
	if _, err := apply(acct, repointStep(shared)); !errors.Is(err, boom) {
		t.Errorf("apply err = %v", err)
	}
	rename, goos = oldRename, oldGOOS

	if target, err := os.Readlink(filepath.Join(acct, "commands")); err != nil || target != other {
		t.Errorf("commands -> %q (%v), want the old link kept at %q", target, err, other)
	}
	if left := tempLinks(t, acct); len(left) != 0 {
		t.Errorf("temporary links left behind: %v", left)
	}
}

func TestRepointSwapsInTheNewLinkAndLeavesNothingElse(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	wrongLink(t, root, shared, acct)

	done, err := apply(acct, repointStep(shared))

	if err != nil || !done {
		t.Fatalf("apply = (%v, %v)", done, err)
	}
	if target, _ := os.Readlink(filepath.Join(acct, "commands")); target != filepath.Join(shared, "commands") {
		t.Errorf("commands -> %q", target)
	}
	if left := tempLinks(t, acct); len(left) != 0 {
		t.Errorf("temporary links left behind: %v", left)
	}
}

func TestRepointToleratesAnIdenticalResultFromElsewhere(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	linkAll(t, acct, shared)
	before := listTree(t, acct)

	done, err := apply(acct, repointStep(shared))

	if err != nil || done {
		t.Errorf("apply = (%v, %v), want (false, nil) for a link that is already right", done, err)
	}
	if after := listTree(t, acct); strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("the tree changed:\nbefore %v\nafter  %v", before, after)
	}
}

func TestRepointNeverReplacesARealFile(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	writeT(t, filepath.Join(acct, "commands"), "mine")

	if _, err := apply(acct, repointStep(shared)); err == nil {
		t.Error("apply replaced a real file")
	}
	if got := readT(t, filepath.Join(acct, "commands")); got != "mine" {
		t.Errorf("the file now holds %q", got)
	}
	if left := tempLinks(t, acct); len(left) != 0 {
		t.Errorf("temporary links left behind: %v", left)
	}
}

func TestRepointFollowsALinkThatWasRemovedMeanwhile(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	mkdirT(t, acct)

	done, err := apply(acct, repointStep(shared))

	if err != nil || !done {
		t.Fatalf("apply = (%v, %v)", done, err)
	}
	if target, _ := os.Readlink(filepath.Join(acct, "commands")); target != filepath.Join(shared, "commands") {
		t.Errorf("commands -> %q", target)
	}
}

func TestSyncsOfOneAccountTakeTurns(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	held, err := fslock.Acquire(filepath.Join(acct, syncLockName), time.Second)
	if err != nil {
		t.Fatal(err)
	}

	finished := make(chan error, 1)
	go func() {
		_, err := Sync(acct, shared)
		finished <- err
	}()
	select {
	case err := <-finished:
		t.Fatalf("Sync ran while the account lock was held (err %v)", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Errorf("Sync: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Sync never ran after the lock was released")
	}
}

func TestSyncGivesUpWhenTheAccountLockIsStuck(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	held, err := fslock.Acquire(filepath.Join(acct, syncLockName), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	old := syncLockTimeout
	syncLockTimeout = 200 * time.Millisecond
	t.Cleanup(func() { syncLockTimeout = old })

	_, err = Sync(acct, shared)

	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("err = %v, want a timeout", err)
	}
}

func TestPlanDoesNotWaitForTheSyncLock(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	held, err := fslock.Acquire(filepath.Join(acct, syncLockName), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()

	start := time.Now()
	mustPlan(t, acct, shared)

	if time.Since(start) > time.Second {
		t.Error("Plan waited for the lock; the health check must stay read-only")
	}
}

func TestParallelSyncsOfOneAccountAgree(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	const n = 8
	reports := make([]Report, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			reports[i], errs[i] = Sync(acct, shared)
		}(i)
	}
	close(start)
	wg.Wait()

	linked := 0
	for i := range reports {
		if errs[i] != nil || reports[i].Err() != nil {
			t.Errorf("sync %d: %v %v", i, errs[i], reports[i].Err())
		}
		linked += len(reports[i].Linked)
	}
	// one of them did the work and the rest found it done
	if linked != len(shareableNames) {
		t.Errorf("%d links were reported across all syncs, want %d", linked, len(shareableNames))
	}
	if steps := mustPlan(t, acct, shared); len(steps) != 0 {
		t.Errorf("the account is not in sync afterwards: %v", steps)
	}
}
