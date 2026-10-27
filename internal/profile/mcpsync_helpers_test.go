package profile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	defA = `{"type":"stdio","command":"a","args":["--x"]}`
	defB = `{"type":"http","url":"https://b.example/mcp"}`
)

// mcpEnv lays out a shared .claude.json and an account folder under one temp
// root. nothing is created yet.
func mcpEnv(t *testing.T) (shared, profileDir string) {
	t.Helper()
	root := t.TempDir()
	return filepath.Join(root, "shared.claude.json"), filepath.Join(root, "acct")
}

func serversJSON(pairs ...string) string {
	var parts []string
	for i := 0; i < len(pairs); i += 2 {
		parts = append(parts, `"`+pairs[i]+`":`+pairs[i+1])
	}
	return `{"mcpServers":{` + strings.Join(parts, ",") + `}}`
}

func mustSyncMCP(t *testing.T, profileDir, shared string) MCPChange {
	t.Helper()
	ch, err := SyncMCP(profileDir, shared)
	if err != nil {
		t.Fatalf("SyncMCP: %v", err)
	}
	return ch
}

// topLevel parses a json object file into its raw top-level values.
func topLevel(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(readT(t, path)), &top); err != nil {
		t.Fatalf("%s is not a json object: %v", path, err)
	}
	return top
}

func sameJSONText(t *testing.T, got, want string) bool {
	t.Helper()
	var g, w bytes.Buffer
	if err := json.Compact(&g, []byte(got)); err != nil {
		t.Fatalf("got is not json: %v", err)
	}
	if err := json.Compact(&w, []byte(want)); err != nil {
		t.Fatalf("want is not json: %v", err)
	}
	return g.String() == w.String()
}

func syncedNames(t *testing.T, profileDir string) []string {
	t.Helper()
	var m struct {
		Synced []string `json:"synced"`
	}
	if err := json.Unmarshal([]byte(readT(t, filepath.Join(profileDir, mcpManifest))), &m); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	return m.Synced
}

// pastMTimes ages every file under dir so a later write shows up as a new mtime.
func pastMTimes(t *testing.T, dir string) {
	t.Helper()
	old := time.Now().Add(-48 * time.Hour)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if err := os.Chtimes(filepath.Join(dir, e.Name()), old, old); err != nil {
			t.Fatal(err)
		}
	}
}

// assertNotWritten fails if anything under dir changed since before was taken
// or if any file there has a fresh mtime. the sync lock file does not count: it
// is made by taking the lock and holds nothing.
func assertNotWritten(t *testing.T, dir string, before []string) {
	t.Helper()
	withoutLock := func(list []string) []string {
		var out []string
		for _, l := range list {
			if !strings.Contains(l, syncLockName) {
				out = append(out, l)
			}
		}
		return out
	}
	if after := listTree(t, dir); !reflect.DeepEqual(withoutLock(before), withoutLock(after)) {
		t.Errorf("folder changed:\nbefore %v\nafter  %v", before, after)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		fi, err := e.Info()
		if e.Name() != syncLockName && err == nil && fi.ModTime().After(time.Now().Add(-time.Hour)) {
			t.Errorf("%s was written", e.Name())
		}
	}
}
