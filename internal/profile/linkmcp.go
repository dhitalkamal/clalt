package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// LinkMCP projects the user-scope MCP servers from srcClaudeJSON (Claude Code's
// ~/.claude.json) into the profile's own .claude.json, so a `clalt run` session
// launched with CLAUDE_CONFIG_DIR pointed at the profile inherits them. Claude
// Code reads MCP servers from $CLAUDE_CONFIG_DIR/.claude.json when that variable
// is set, which is why the profile needs its own copy.
//
// It merges rather than overwrites: a server name the profile already defines is
// left untouched (the profile can diverge), and any other top-level keys already
// in the profile's .claude.json are preserved. Returns the names it added; a
// no-op returns nil so the caller can stay quiet.
//
// Note: the profile file is written as ".claude.json" (dot-prefixed) inside the
// profile dir, matching how CLAUDE_CONFIG_DIR relocates ~/.claude.json. If a
// future Claude Code release reads a different name under CLAUDE_CONFIG_DIR, this
// is the single spot to adjust.
func LinkMCP(profileDir, srcClaudeJSON string) ([]string, error) {
	srcServers, err := readServers(srcClaudeJSON)
	if err != nil || len(srcServers) == 0 {
		return nil, err // missing source or no servers: nothing to do
	}

	dstPath := filepath.Join(profileDir, ".claude.json")
	top, err := readTopLevel(dstPath)
	if err != nil {
		return nil, err
	}
	dstServers := map[string]json.RawMessage{}
	if raw, ok := top["mcpServers"]; ok {
		_ = json.Unmarshal(raw, &dstServers)
	}

	var added []string
	for name, def := range srcServers {
		if _, exists := dstServers[name]; exists {
			continue // never clobber a server the profile already defines
		}
		dstServers[name] = def
		added = append(added, name)
	}
	if len(added) == 0 {
		return nil, nil // already in sync: no write, idempotent
	}
	sort.Strings(added)

	mergedServers, err := json.Marshal(dstServers)
	if err != nil {
		return nil, err
	}
	top["mcpServers"] = mergedServers
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(dstPath, out, 0o600); err != nil {
		return nil, err
	}
	return added, nil
}

// readServers returns the top-level mcpServers object from a .claude.json, or an
// empty map when the file is absent or has none.
func readServers(path string) (map[string]json.RawMessage, error) {
	top, err := readTopLevel(path)
	if err != nil {
		return nil, err
	}
	raw, ok := top["mcpServers"]
	if !ok {
		return nil, nil
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(raw, &servers); err != nil {
		return nil, err
	}
	return servers, nil
}

// readTopLevel parses a JSON object into its top-level keys, preserving each
// value verbatim. A missing file yields an empty map (not an error) so callers
// can treat "no file" and "empty object" the same.
func readTopLevel(path string) (map[string]json.RawMessage, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]json.RawMessage{}, nil
		}
		return nil, err
	}
	top := map[string]json.RawMessage{}
	if len(raw) == 0 {
		return top, nil
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, err
	}
	return top, nil
}
