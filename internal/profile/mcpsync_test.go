package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSyncMCPAddsSharedServers(t *testing.T) {
	shared, profileDir := mcpEnv(t)
	putJSON(t, shared, serversJSON("b", defB, "a", defA))

	ch := mustSyncMCP(t, profileDir, shared)

	if !reflect.DeepEqual(ch, MCPChange{Added: []string{"a", "b"}}) {
		t.Errorf("change = %+v", ch)
	}
	got := mcpServersIn(t, profileDir)
	if !sameJSONText(t, string(got["a"]), defA) || !sameJSONText(t, string(got["b"]), defB) {
		t.Errorf("servers = %s", got)
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(syncedNames(t, profileDir), want) {
		t.Errorf("manifest = %v, want %v", syncedNames(t, profileDir), want)
	}
	if runtime.GOOS != "windows" {
		for _, f := range []string{".claude.json", mcpManifest} {
			fi, err := os.Stat(filepath.Join(profileDir, f))
			if err != nil {
				t.Fatal(err)
			}
			if fi.Mode().Perm() != 0o600 {
				t.Errorf("%s: mode %v, want 0600", f, fi.Mode().Perm())
			}
		}
	}
}

func TestSyncMCPKeepsOtherKeysVerbatim(t *testing.T) {
	shared, profileDir := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA))
	own := `{"numStartups":7,"big":12345678901234567890,"html":"<a href=\"x\">&</a>","projects":{"/p":{"history":[1,2.50,null]}},"mcpServers":{"mine":{"command":"m"}}}`
	putJSON(t, filepath.Join(profileDir, ".claude.json"), own)

	mustSyncMCP(t, profileDir, shared)

	var before map[string]json.RawMessage
	if err := json.Unmarshal([]byte(own), &before); err != nil {
		t.Fatal(err)
	}
	after := topLevel(t, filepath.Join(profileDir, ".claude.json"))
	for _, k := range []string{"numStartups", "big", "html", "projects"} {
		if !sameJSONText(t, string(after[k]), string(before[k])) {
			t.Errorf("%s changed: %s -> %s", k, before[k], after[k])
		}
	}
	// no float rounding and no < escaping on the way through
	raw := readT(t, filepath.Join(profileDir, ".claude.json"))
	for _, want := range []string{`12345678901234567890`, `<a href=\"x\">&</a>`, `2.50`} {
		if !strings.Contains(raw, want) {
			t.Errorf("rewritten file lost %s", want)
		}
	}
	if servers := mcpServersIn(t, profileDir); len(servers) != 2 {
		t.Errorf("servers = %s, want mine and a", servers)
	}
}

func TestSyncMCPSharedWinsOnANameClash(t *testing.T) {
	shared, profileDir := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA))
	putJSON(t, filepath.Join(profileDir, ".claude.json"), serversJSON("a", `{"command":"mine"}`))

	ch := mustSyncMCP(t, profileDir, shared)

	if !reflect.DeepEqual(ch, MCPChange{Updated: []string{"a"}, Overwrote: []string{"a"}}) {
		t.Errorf("change = %+v", ch)
	}
	if got := mcpServersIn(t, profileDir); !sameJSONText(t, string(got["a"]), defA) {
		t.Errorf("a = %s, want the shared definition", got["a"])
	}
}

func TestSyncMCPRemovesServersGoneFromShared(t *testing.T) {
	shared, profileDir := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA, "b", defB))
	mustSyncMCP(t, profileDir, shared)
	putJSON(t, shared, serversJSON("a", defA))

	ch := mustSyncMCP(t, profileDir, shared)

	if !reflect.DeepEqual(ch, MCPChange{Removed: []string{"b"}}) {
		t.Errorf("change = %+v", ch)
	}
	if got := mcpServersIn(t, profileDir); len(got) != 1 || got["a"] == nil {
		t.Errorf("servers = %s, want only a", got)
	}
	if !reflect.DeepEqual(syncedNames(t, profileDir), []string{"a"}) {
		t.Errorf("manifest = %v", syncedNames(t, profileDir))
	}
}

func TestSyncMCPKeepsServersTheAccountAddedItself(t *testing.T) {
	shared, profileDir := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA))
	putJSON(t, filepath.Join(profileDir, ".claude.json"), serversJSON("mine", `{"command":"m"}`))
	mustSyncMCP(t, profileDir, shared)
	putJSON(t, shared, `{}`)

	ch := mustSyncMCP(t, profileDir, shared)

	if !reflect.DeepEqual(ch, MCPChange{Removed: []string{"a"}}) {
		t.Errorf("change = %+v", ch)
	}
	if got := mcpServersIn(t, profileDir); len(got) != 1 || got["mine"] == nil {
		t.Errorf("servers = %s, want only mine", got)
	}
	if len(syncedNames(t, profileDir)) != 0 {
		t.Errorf("manifest = %v, want empty", syncedNames(t, profileDir))
	}
}

func TestSyncMCPWithNothingToDoWritesNoConfig(t *testing.T) {
	shared, profileDir := mcpEnv(t)
	// no shared file and nothing synced before
	if ch := mustSyncMCP(t, profileDir, shared); !ch.Empty() {
		t.Errorf("change = %+v", ch)
	}
	// a shared file without servers
	putJSON(t, shared, `{"numStartups":1,"mcpServers":null}`)
	if ch := mustSyncMCP(t, profileDir, shared); !ch.Empty() {
		t.Errorf("change = %+v", ch)
	}
	for _, f := range []string{".claude.json", mcpManifest} {
		if present(filepath.Join(profileDir, f)) {
			t.Errorf("%s was written with nothing to sync", f)
		}
	}
}

func TestSyncMCPSecondRunChangesNothing(t *testing.T) {
	shared, profileDir := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA, "b", defB))
	mustSyncMCP(t, profileDir, shared)
	pastMTimes(t, profileDir)
	before := listTree(t, profileDir)

	if ch := mustSyncMCP(t, profileDir, shared); !ch.Empty() {
		t.Errorf("second run changed %+v", ch)
	}
	assertNotWritten(t, profileDir, before)
}

func TestSyncMCPIgnoresFormattingWhenComparing(t *testing.T) {
	shared, profileDir := mcpEnv(t)
	putJSON(t, shared, `{"mcpServers":{"a":{"type":"stdio","command":"a","args":["--x"]}}}`)
	// same server, other key order and spacing, as claude would rewrite it
	putJSON(t, filepath.Join(profileDir, ".claude.json"),
		"{\n  \"mcpServers\": {\n    \"a\": {\n      \"args\": [\"--x\"],\n      \"command\": \"a\",\n      \"type\": \"stdio\"\n    }\n  }\n}\n")
	pastMTimes(t, profileDir)

	if ch := mustSyncMCP(t, profileDir, shared); !ch.Empty() {
		t.Errorf("change = %+v, want none for an equal server", ch)
	}
	if fi, _ := os.Stat(filepath.Join(profileDir, ".claude.json")); !fi.ModTime().Before(time.Now().Add(-time.Hour)) {
		t.Error("rewrote the account's .claude.json for an equal server")
	}
}

func TestSyncMCPHealsAStaleManifestWithoutTouchingTheConfig(t *testing.T) {
	shared, profileDir := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA))
	putJSON(t, filepath.Join(profileDir, ".claude.json"), serversJSON("a", defA))
	// an earlier run died between its two writes: the config is done, the manifest is old
	putJSON(t, filepath.Join(profileDir, mcpManifest), `{"synced":["gone"]}`)
	pastMTimes(t, profileDir)

	if ch := mustSyncMCP(t, profileDir, shared); !ch.Empty() {
		t.Errorf("change = %+v, want none: nothing to remove from the config", ch)
	}
	if !reflect.DeepEqual(syncedNames(t, profileDir), []string{"a"}) {
		t.Errorf("manifest = %v, want [a]", syncedNames(t, profileDir))
	}
	if fi, _ := os.Stat(filepath.Join(profileDir, ".claude.json")); !fi.ModTime().Before(time.Now().Add(-time.Hour)) {
		t.Error("rewrote .claude.json when only the manifest was stale")
	}
}

func TestSyncMCPComparesNumbersExactly(t *testing.T) {
	shared, profileDir := mcpEnv(t)
	// both round to the same float64
	putJSON(t, shared, serversJSON("a", `{"id":12345678901234567890}`))
	putJSON(t, filepath.Join(profileDir, ".claude.json"), serversJSON("a", `{"id":12345678901234567891}`))

	ch := mustSyncMCP(t, profileDir, shared)

	if !reflect.DeepEqual(ch, MCPChange{Updated: []string{"a"}, Overwrote: []string{"a"}}) {
		t.Errorf("change = %+v, want a updated", ch)
	}
}
