package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// the marketplace lists the plugin by a path that exists, under the same name
// as the plugin's own manifest, and the plugin runs clalt mcp from PATH.
func TestPluginPackagingAgrees(t *testing.T) {
	var mk struct {
		Name    string
		Plugins []struct{ Name, Source string }
	}
	readJSON(t, ".claude-plugin/marketplace.json", &mk)
	if mk.Name == "" || len(mk.Plugins) != 1 {
		t.Fatalf("marketplace = %+v", mk)
	}
	root := filepath.Clean(mk.Plugins[0].Source)
	var man struct{ Name, Version string }
	readJSON(t, filepath.Join(root, ".claude-plugin", "plugin.json"), &man)
	if man.Name != mk.Plugins[0].Name || man.Version == "" {
		t.Errorf("manifest %+v does not match the marketplace entry %+v", man, mk.Plugins[0])
	}
	var mcp struct {
		McpServers map[string]struct {
			Command string
			Args    []string
		}
	}
	readJSON(t, filepath.Join(root, ".mcp.json"), &mcp)
	s, ok := mcp.McpServers["clalt"]
	if !ok || s.Command != "clalt" || len(s.Args) != 1 || s.Args[0] != "mcp" {
		t.Errorf(".mcp.json = %+v, want clalt mcp", mcp)
	}
}
