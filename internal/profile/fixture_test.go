package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// small file helpers for these tests. they have names of their own so the tests
// do not lean on the helpers of the older link tests.
func mkdirT(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeT(t *testing.T, p, body string) {
	t.Helper()
	mkdirT(t, filepath.Dir(p))
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readT(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// present is true for anything at p, a dangling link included.
func present(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func putJSON(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// mcpServersIn returns the servers in an account's .claude.json, nil when the
// file is missing.
func mcpServersIn(t *testing.T, profileDir string) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(profileDir, ".claude.json"))
	if err != nil {
		return nil
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("the account's .claude.json is not valid json: %v", err)
	}
	var servers map[string]json.RawMessage
	if m, ok := top["mcpServers"]; ok {
		_ = json.Unmarshal(m, &servers)
	}
	return servers
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
}

// sharedFixture builds a shared home with every kind of entry the rules care
// about. the shareable ones are agents, projects, commands, CLAUDE.md,
// history.jsonl and settings.json.
func sharedFixture(t *testing.T) (root, shared string) {
	t.Helper()
	root = t.TempDir()
	shared = filepath.Join(root, "shared")
	for _, d := range []string{"commands", "agents", "projects", "sessions", "cache"} {
		mkdirT(t, filepath.Join(shared, d))
	}
	for _, f := range []string{"settings.json", "CLAUDE.md", "history.jsonl", ".credentials.json", ".claude.json", "stats-cache.json"} {
		writeT(t, filepath.Join(shared, f), "x")
	}
	return root, shared
}

var shareableNames = []string{"CLAUDE.md", "agents", "commands", "history.jsonl", "projects", "settings.json"}

// linkAll links every shareable name the way a sync would.
func linkAll(t *testing.T, profileDir, shared string) {
	t.Helper()
	for _, n := range shareableNames {
		mustSymlink(t, filepath.Join(shared, n), filepath.Join(profileDir, n))
	}
}

func stepNames(steps []Step) []string {
	var out []string
	for _, s := range steps {
		out = append(out, s.Name)
	}
	return out
}

func stepMap(steps []Step) map[string]Step {
	m := map[string]Step{}
	for _, s := range steps {
		m[s.Name] = s
	}
	return m
}

func mustPlan(t *testing.T, profileDir, shared string) []Step {
	t.Helper()
	steps, err := Plan(profileDir, shared)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	// one name never gets two steps
	seen := map[string]bool{}
	for _, st := range steps {
		if seen[st.Name] {
			t.Errorf("%s is planned twice", st.Name)
		}
		seen[st.Name] = true
	}
	return steps
}

// listTree records every path under dir with its type, links included.
func listTree(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		out = append(out, p+" "+d.Type().String())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func mustReadDir(t *testing.T, dir string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}
