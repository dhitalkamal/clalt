package profile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
)

// readObject parses a json object file into raw top-level values, which keeps
// every value as written, and returns the bytes it read. a missing file is an
// empty object with nil bytes; an empty file, bad json or anything but an
// object is an error.
func readObject(path string) (raw []byte, top map[string]json.RawMessage, err error) {
	raw, err = os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	if top == nil {
		return nil, nil, fmt.Errorf("%s: not a json object", path)
	}
	return raw, top, nil
}

// serversOf returns the mcpServers object of a parsed config; absent or null
// means none. the map is a copy, so the caller can edit it freely.
func serversOf(top map[string]json.RawMessage, path string) (map[string]json.RawMessage, error) {
	servers := map[string]json.RawMessage{}
	raw, ok := top["mcpServers"]
	if !ok {
		return servers, nil
	}
	if err := json.Unmarshal(raw, &servers); err != nil {
		return nil, fmt.Errorf("%s: mcpServers is not an object", path)
	}
	if servers == nil {
		servers = map[string]json.RawMessage{}
	}
	return servers, nil
}

func readManifest(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m manifestFile
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return m.Synced, nil
}

// encodeJSON leaves <, > and & as they are: escaping them would change the
// bytes of server definitions that carry URLs.
func encodeJSON(v any, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if indent != "" {
		enc.SetIndent("", indent)
	}
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// sameJSON compares two values by meaning, not bytes: the account's file may be
// rewritten with other spacing or key order. numbers stay as text so two large
// integers never compare equal by rounding.
func sameJSON(a, b json.RawMessage) bool {
	var x, y any
	return decodeExact(a, &x) && decodeExact(b, &y) && reflect.DeepEqual(x, y)
}

func decodeExact(raw []byte, v any) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec.Decode(v) == nil
}

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func sortedSet(names []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// checkMCPPaths rejects empty and relative paths for the same reason checkDirs
// does: they would be read from wherever the caller happens to be.
func checkMCPPaths(profileDir, sharedClaudeJSON string) error {
	for _, d := range []struct{ what, path string }{{"account folder", profileDir}, {"shared claude config", sharedClaudeJSON}} {
		switch {
		case d.path == "":
			return fmt.Errorf("no %s given", d.what)
		case !filepath.IsAbs(d.path):
			return fmt.Errorf("the %s must be an absolute path, got %q", d.what, d.path)
		}
	}
	return nil
}
