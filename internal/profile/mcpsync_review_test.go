package profile

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/fslock"
	"github.com/dhitalkamal/clalt/internal/fsutil"
)

func TestSyncMCPRefusesWhenTheSharedConfigIsMissingButServersWereSynced(t *testing.T) {
	shared, acct := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA, "b", defB))
	mustSyncMCP(t, acct, shared)
	if err := os.Remove(shared); err != nil {
		t.Fatal(err)
	}
	pastMTimes(t, acct)
	before := listTree(t, acct)

	for name, call := range map[string]func(string, string) (MCPChange, error){"SyncMCP": SyncMCP, "PlanMCP": PlanMCP} {
		ch, err := call(acct, shared)
		if err == nil || !ch.Empty() {
			t.Errorf("%s = %+v, %v, want an error and no change", name, ch, err)
			continue
		}
		if !strings.Contains(err.Error(), shared) || !strings.Contains(err.Error(), "2") {
			t.Errorf("%s error %q should name the file and the count", name, err)
		}
	}
	assertNotWritten(t, acct, before)
	if got := mcpServersIn(t, acct); len(got) != 2 {
		t.Errorf("servers = %s, want both kept", got)
	}
}

func TestSyncMCPStillRemovesWhenTheSharedConfigExistsWithoutServers(t *testing.T) {
	shared, acct := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA))
	mustSyncMCP(t, acct, shared)
	putJSON(t, shared, `{"numStartups":3}`)

	if ch := mustSyncMCP(t, acct, shared); !reflect.DeepEqual(ch, MCPChange{Removed: []string{"a"}}) {
		t.Errorf("change = %+v", ch)
	}
}

// changeOnce makes afterPlan rewrite the account config the first times it is
// called, the way claude would while a sync is running.
func changeOnce(t *testing.T, path string, bodies ...string) *int {
	t.Helper()
	calls := 0
	old := afterPlan
	afterPlan = func() {
		if calls < len(bodies) {
			putJSON(t, path, bodies[calls])
		}
		calls++
	}
	t.Cleanup(func() { afterPlan = old })
	return &calls
}

func TestSyncMCPReplansWhenTheAccountConfigChangedMeanwhile(t *testing.T) {
	shared, acct := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA))
	config := filepath.Join(acct, ".claude.json")
	putJSON(t, config, `{"numStartups":1}`)
	calls := changeOnce(t, config, `{"numStartups":2,"tipsShown":true}`)

	ch := mustSyncMCP(t, acct, shared)

	if !reflect.DeepEqual(ch, MCPChange{Added: []string{"a"}}) {
		t.Errorf("change = %+v", ch)
	}
	top := topLevel(t, config)
	if string(top["numStartups"]) != "2" || string(top["tipsShown"]) != "true" || top["mcpServers"] == nil {
		t.Errorf("config = %s, want claude's newer keys and the server", readT(t, config))
	}
	if *calls != 2 {
		t.Errorf("planned %d times, want 2", *calls)
	}
}

func TestSyncMCPGivesUpWhenTheAccountConfigKeepsChanging(t *testing.T) {
	shared, acct := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA))
	config := filepath.Join(acct, ".claude.json")
	putJSON(t, config, `{"n":0}`)
	calls := changeOnce(t, config, `{"n":1}`, `{"n":2}`, `{"n":3}`, `{"n":4}`)

	_, err := SyncMCP(acct, shared)

	if err == nil || !strings.Contains(err.Error(), config) {
		t.Errorf("err = %v, want one naming %s", err, config)
	}
	if *calls != 3 {
		t.Errorf("planned %d times, want 3", *calls)
	}
	if got := readT(t, config); got != `{"n":3}` {
		t.Errorf("the config holds %q: only claude's own writes may be there", got)
	}
	if present(filepath.Join(acct, mcpManifest)) {
		t.Error("a manifest was written for a sync that did not happen")
	}
}

func TestSyncMCPHoldsTheAccountLockForTheWholeSync(t *testing.T) {
	shared, acct := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA))
	held, err := fslock.Acquire(filepath.Join(acct, syncLockName), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := SyncMCP(acct, shared)
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("SyncMCP ran while the lock was held (err %v)", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("SyncMCP: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SyncMCP never ran after the lock was released")
	}
}

func TestPlanMCPDoesNotTakeTheLock(t *testing.T) {
	shared, acct := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA))
	held, err := fslock.Acquire(filepath.Join(acct, syncLockName), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()

	start := time.Now()
	if _, err := PlanMCP(acct, shared); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second {
		t.Error("PlanMCP waited for the lock")
	}
}

func TestSyncMCPReportsWhichAccountServersItOverwrote(t *testing.T) {
	shared, acct := mcpEnv(t)
	putJSON(t, shared, serversJSON("mine", defA, "managed", defB, "same", `{"command":"s"}`))
	// the account has its own "mine" and "same" (equal to the shared one), and
	// clalt mirrored "managed" earlier
	putJSON(t, filepath.Join(acct, ".claude.json"), serversJSON(
		"mine", `{"command":"mine"}`, "managed", `{"command":"old"}`, "same", `{"command":"s"}`))
	putJSON(t, filepath.Join(acct, mcpManifest), `{"synced":["managed"]}`)

	ch := mustSyncMCP(t, acct, shared)

	want := MCPChange{Updated: []string{"managed", "mine"}, Overwrote: []string{"mine"}}
	if !reflect.DeepEqual(ch, want) {
		t.Errorf("change = %+v, want %+v", ch, want)
	}
}

func TestSyncMCPReturnsTheChangeWithTheErrorWhenTheManifestWriteFails(t *testing.T) {
	shared, acct := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA))
	boom := errors.New("disk full")
	old := writeFile
	writeFile = func(path string, data []byte, perm os.FileMode) error {
		if filepath.Base(path) == mcpManifest {
			return boom
		}
		return fsutil.WriteFileAtomic(path, data, perm)
	}
	t.Cleanup(func() { writeFile = old })

	ch, err := SyncMCP(acct, shared)

	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	// the account's config did change, and the caller has to be able to say so
	if !reflect.DeepEqual(ch, MCPChange{Added: []string{"a"}}) {
		t.Errorf("change = %+v", ch)
	}
	if got := mcpServersIn(t, acct); got["a"] == nil {
		t.Error("the config was not written")
	}
}

func TestSyncMCPRefusesToWriteThroughASymlinkedAccountConfig(t *testing.T) {
	shared, acct := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA))
	real := filepath.Join(filepath.Dir(acct), "real.claude.json")
	putJSON(t, real, `{"numStartups":1}`)
	mustSymlink(t, real, filepath.Join(acct, ".claude.json"))

	for name, call := range map[string]func(string, string) (MCPChange, error){"SyncMCP": SyncMCP, "PlanMCP": PlanMCP} {
		if _, err := call(acct, shared); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Errorf("%s err = %v, want a refusal that says symlink", name, err)
		}
	}
	if got := readT(t, real); got != `{"numStartups":1}` {
		t.Errorf("the file behind the link now holds %q", got)
	}
	if fi, err := os.Lstat(filepath.Join(acct, ".claude.json")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Error("the symlink was replaced")
	}
}

func TestSyncMCPAcceptsASymlinkedAccountConfigThatNeedsNoChange(t *testing.T) {
	shared, acct := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA))
	real := filepath.Join(filepath.Dir(acct), "real.claude.json")
	putJSON(t, real, serversJSON("a", defA))
	mustSymlink(t, real, filepath.Join(acct, ".claude.json"))

	if ch, err := SyncMCP(acct, shared); err != nil || !ch.Empty() {
		t.Errorf("SyncMCP = %+v, %v, want nothing to do", ch, err)
	}
}
