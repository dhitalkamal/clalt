package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// readMCP returns the server names present in a profile's .claude.json.
func readMCP(t *testing.T, profileDir string) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(profileDir, ".claude.json"))
	if err != nil {
		return nil
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("profile .claude.json invalid: %v", err)
	}
	var servers map[string]json.RawMessage
	if m, ok := top["mcpServers"]; ok {
		_ = json.Unmarshal(m, &servers)
	}
	return servers
}

func writeJSON(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLinkMCPProjectsUserServers(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, ".claude.json")
	profileDir := filepath.Join(root, "profile")
	writeJSON(t, src, `{"mcpServers":{"a":{"type":"stdio","command":"a"},"b":{"type":"http","url":"u"}},"numStartups":5}`)

	added, err := LinkMCP(profileDir, src)
	if err != nil {
		t.Fatalf("LinkMCP: %v", err)
	}
	if len(added) != 2 {
		t.Fatalf("added = %v, want 2 servers", added)
	}
	got := readMCP(t, profileDir)
	if _, ok := got["a"]; !ok {
		t.Error("server a not projected")
	}
	if _, ok := got["b"]; !ok {
		t.Error("server b not projected")
	}
}

func TestLinkMCPIsIdempotent(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, ".claude.json")
	profileDir := filepath.Join(root, "profile")
	writeJSON(t, src, `{"mcpServers":{"a":{"command":"a"}}}`)

	if _, err := LinkMCP(profileDir, src); err != nil {
		t.Fatal(err)
	}
	added, err := LinkMCP(profileDir, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 0 {
		t.Errorf("second link added %v, want nothing", added)
	}
}

func TestLinkMCPDoesNotClobberProfileServer(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, ".claude.json")
	profileDir := filepath.Join(root, "profile")
	writeJSON(t, src, `{"mcpServers":{"a":{"command":"from-source"},"b":{"command":"b"}}}`)
	// profile already defines its own "a" and an unrelated top-level key
	writeJSON(t, filepath.Join(profileDir, ".claude.json"),
		`{"mcpServers":{"a":{"command":"profile-own"}},"customKey":42}`)

	added, err := LinkMCP(profileDir, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 1 || added[0] != "b" {
		t.Fatalf("added = %v, want only [b]", added)
	}
	got := readMCP(t, profileDir)
	var a struct {
		Command string `json:"command"`
	}
	_ = json.Unmarshal(got["a"], &a)
	if a.Command != "profile-own" {
		t.Errorf("profile server a was clobbered: %q", a.Command)
	}
	// unrelated top-level key must survive
	raw, _ := os.ReadFile(filepath.Join(profileDir, ".claude.json"))
	var top map[string]json.RawMessage
	_ = json.Unmarshal(raw, &top)
	if _, ok := top["customKey"]; !ok {
		t.Error("unrelated top-level key customKey was dropped")
	}
}

func TestLinkMCPNoSourceIsNoOp(t *testing.T) {
	root := t.TempDir()
	profileDir := filepath.Join(root, "profile")

	added, err := LinkMCP(profileDir, filepath.Join(root, "missing.json"))
	if err != nil {
		t.Fatalf("missing source should be a no-op, got %v", err)
	}
	if len(added) != 0 {
		t.Errorf("added = %v, want nothing", added)
	}
	if _, err := os.Stat(filepath.Join(profileDir, ".claude.json")); err == nil {
		t.Error("no profile .claude.json should be written when source is absent")
	}
}
