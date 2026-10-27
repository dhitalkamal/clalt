package profile

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// these cover unchangedSince: the account's file is looked at again right
// before the write, and a file that was missing, gone or empty when the plan
// was made counts as changed just like one with other content.

func TestSyncMCPReplansWhenTheAccountConfigAppearsMeanwhile(t *testing.T) {
	shared, acct := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA))
	config := filepath.Join(acct, ".claude.json")
	// a fresh account: claude creates its file while clalt is still planning
	calls := changeOnce(t, config, `{"userID":"u-1","numStartups":1}`)

	ch := mustSyncMCP(t, acct, shared)

	if !reflect.DeepEqual(ch, MCPChange{Added: []string{"a"}}) {
		t.Errorf("change = %+v", ch)
	}
	top := topLevel(t, config)
	if string(top["userID"]) != `"u-1"` || string(top["numStartups"]) != "1" || top["mcpServers"] == nil {
		t.Errorf("config = %s, want claude's keys and the server", readT(t, config))
	}
	if *calls != 2 {
		t.Errorf("planned %d times, want 2", *calls)
	}
}

func TestSyncMCPReplansWhenTheAccountConfigVanishesMeanwhile(t *testing.T) {
	shared, acct := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA))
	config := filepath.Join(acct, ".claude.json")
	putJSON(t, config, `{"numStartups":1}`)
	calls := 0
	old := afterPlan
	afterPlan = func() {
		if calls == 0 {
			if err := os.Remove(config); err != nil {
				t.Error(err)
			}
		}
		calls++
	}
	t.Cleanup(func() { afterPlan = old })

	mustSyncMCP(t, acct, shared)

	top := topLevel(t, config)
	if _, ok := top["numStartups"]; ok || top["mcpServers"] == nil {
		t.Errorf("config = %s, want only the server: the old keys were deleted on purpose", readT(t, config))
	}
	if calls != 2 {
		t.Errorf("planned %d times, want 2", calls)
	}
}

func TestSyncMCPDoesNotWriteOverAFileThatAppearedEmpty(t *testing.T) {
	shared, acct := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA))
	config := filepath.Join(acct, ".claude.json")
	changeOnce(t, config, "")

	_, err := SyncMCP(acct, shared)

	if err == nil {
		t.Error("SyncMCP wrote over a file it had not looked at")
	}
	if got := readT(t, config); got != "" {
		t.Errorf("the file now holds %q", got)
	}
	if present(filepath.Join(acct, mcpManifest)) {
		t.Error("a manifest was written for a sync that did not happen")
	}
}
