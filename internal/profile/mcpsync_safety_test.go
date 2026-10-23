package profile

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSyncMCPAbortsWithoutWritingOnBadInput(t *testing.T) {
	cases := []struct {
		name        string
		shared      string // body of the shared file, which always exists
		local       string // account config body, empty means absent
		manifest    string
		sharedIsDir bool
	}{
		{name: "shared is not json", shared: `{"mcpServers":{"a":`},
		{name: "shared is empty", shared: ""},
		{name: "shared is null", shared: `null`},
		{name: "shared is an array", shared: `[]`},
		{name: "shared servers are not an object", shared: `{"mcpServers":["a"]}`},
		{name: "shared cannot be read", sharedIsDir: true},
		{name: "account config is not json", shared: serversJSON("a", defA), local: `{"mcpServers":`},
		{name: "account config is empty", shared: serversJSON("a", defA), local: " \n"},
		{name: "account servers are not an object", shared: serversJSON("a", defA), local: `{"mcpServers":"x"}`},
		{name: "manifest is not json", shared: serversJSON("a", defA), manifest: `{"synced":`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			shared, profileDir := mcpEnv(t)
			if c.sharedIsDir {
				mkdirT(t, shared)
			} else {
				putJSON(t, shared, c.shared)
			}
			mkdirT(t, profileDir)
			if c.local != "" {
				putJSON(t, filepath.Join(profileDir, ".claude.json"), c.local)
			}
			if c.manifest != "" {
				putJSON(t, filepath.Join(profileDir, mcpManifest), c.manifest)
			}
			pastMTimes(t, profileDir)
			before := listTree(t, profileDir)

			if _, err := SyncMCP(profileDir, shared); err == nil {
				t.Fatal("SyncMCP accepted bad input")
			}
			if _, err := PlanMCP(profileDir, shared); err == nil {
				t.Error("PlanMCP accepted bad input")
			}
			assertNotWritten(t, profileDir, before)
		})
	}
}

func TestSyncMCPErrorsNeverEchoServerDefinitions(t *testing.T) {
	shared, profileDir := mcpEnv(t)
	secret := "s3cr3t-token-value"
	putJSON(t, shared, `{"mcpServers":{"a":{"env":{"TOKEN":"`+secret+`"}}},"broken":}`)
	_, err := SyncMCP(profileDir, shared)
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error leaks a server definition: %v", err)
	}
	putJSON(t, shared, `{"mcpServers":{"a":{"env":{"TOKEN":"`+secret+`"}}}}`)
	putJSON(t, filepath.Join(profileDir, ".claude.json"), `{"mcpServers":{"a":{"env":{"TOKEN":"`+secret+`"}}},"x":}`)
	if _, err = SyncMCP(profileDir, shared); err == nil || strings.Contains(err.Error(), secret) {
		t.Errorf("error = %v, want one without the definition", err)
	}
}

func TestPlanMCPReportsTheChangeAndWritesNothing(t *testing.T) {
	shared, profileDir := mcpEnv(t)
	putJSON(t, shared, serversJSON("new", defA, "clash", defB))
	putJSON(t, filepath.Join(profileDir, ".claude.json"), serversJSON("clash", `{"command":"mine"}`, "stale", `{"command":"s"}`))
	putJSON(t, filepath.Join(profileDir, mcpManifest), `{"synced":["stale"]}`)
	pastMTimes(t, profileDir)
	before := listTree(t, profileDir)

	got, err := PlanMCP(profileDir, shared)
	if err != nil {
		t.Fatal(err)
	}

	want := MCPChange{Added: []string{"new"}, Updated: []string{"clash"}, Removed: []string{"stale"}, Overwrote: []string{"clash"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PlanMCP = %+v, want %+v", got, want)
	}
	assertNotWritten(t, profileDir, before)

	// and a sync then does exactly what the plan said
	if applied := mustSyncMCP(t, profileDir, shared); !reflect.DeepEqual(applied, want) {
		t.Errorf("SyncMCP = %+v, want %+v", applied, want)
	}
	if again, _ := PlanMCP(profileDir, shared); !again.Empty() {
		t.Errorf("PlanMCP after a sync = %+v, want nothing", again)
	}
}

func TestSyncMCPLeavesNoTempFilesBehind(t *testing.T) {
	shared, profileDir := mcpEnv(t)
	putJSON(t, shared, serversJSON("a", defA))
	mustSyncMCP(t, profileDir, shared)
	entries, err := os.ReadDir(profileDir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	// the listing comes back sorted by name
	if want := []string{mcpManifest, syncLockName, ".claude.json"}; !reflect.DeepEqual(names, want) {
		t.Errorf("folder holds %v, want %v", names, want)
	}
}
