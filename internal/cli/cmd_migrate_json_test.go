package cli

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func keysOf(m map[string]any) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// wantKeys compares the raw json keys, so a renamed or leaked field shows up
// even when the struct it decodes into would hide it.
func wantKeys(t *testing.T, label string, m map[string]any, want ...string) {
	t.Helper()
	if got := keysOf(m); !reflect.DeepEqual(got, want) {
		t.Errorf("%s keys = %v, want %v", label, got, want)
	}
}

func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%v is not an object", v)
	}
	return m
}

func TestMigrateJSONHasTheDocumentedKeys(t *testing.T) {
	profiles, shared := migrateHome(t)
	elsewhere := filepath.Join(t.TempDir(), "elsewhere")
	migrateWrite(t, filepath.Join(elsewhere, "x.jsonl"), "x")
	migrateWrite(t, filepath.Join(shared, "-p", "clash.jsonl"), "theirs")
	alpha := filepath.Join(profiles, "alpha", "projects", "-p")
	migrateWrite(t, filepath.Join(alpha, "a.jsonl"), "first")
	migrateWrite(t, filepath.Join(alpha, "clash.jsonl"), "mine")
	mustSymlink(t, filepath.Join(elsewhere, "x.jsonl"), filepath.Join(alpha, "ln.jsonl"))
	mustSymlink(t, elsewhere, filepath.Join(profiles, "beta", "projects"))
	migrateWrite(t, filepath.Join(profiles, "gamma", "settings.json"), "{}")

	code, out, errOut := runMigrate(t, "--json")
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	wantKeys(t, "report", doc, "dest", "dryRun", "profiles", "source", "total")
	wantKeys(t, "total", asMap(t, doc["total"]),
		"alreadyThere", "bytes", "conflicts", "copied", "failed", "linked", "skipped", "writtenInPlace")

	byName := map[string]map[string]any{}
	for _, p := range doc["profiles"].([]any) {
		m := asMap(t, p)
		byName[m["name"].(string)] = m
	}
	folderKeys := []string{"alreadyThere", "bytes", "conflicts", "copied", "failed", "name", "skipped", "skippedEntries", "status", "writtenInPlace"}
	wantKeys(t, "a folder profile", byName["alpha"], folderKeys...)
	wantKeys(t, "a profile with nothing", byName["gamma"], folderKeys...)
	wantKeys(t, "a link profile", byName["beta"],
		"alreadyThere", "bytes", "conflicts", "copied", "failed", "linkOf", "name", "skipped", "skippedEntries", "status", "target", "writtenInPlace")

	alphaDoc := byName["alpha"]
	wantKeys(t, "a conflict", asMap(t, alphaDoc["conflicts"].([]any)[0]), "path", "reason")
	wantKeys(t, "a skipped entry", asMap(t, alphaDoc["skippedEntries"].([]any)[0]), "kind", "path", "target")
	for field, want := range map[string]any{"status": "folder"} {
		if alphaDoc[field] != want {
			t.Errorf("alpha %s = %v, want %v", field, alphaDoc[field], want)
		}
	}
	if byName["beta"]["status"] != "link" || byName["beta"]["linkOf"] != "projects" || byName["gamma"]["status"] != "nothing" {
		t.Errorf("statuses wrong: %v", byName)
	}
	// an empty list is [] and never null
	if !strings.Contains(out, `"failed": []`) || strings.Contains(out, "null") {
		t.Errorf("empty lists should be [] and nothing should be null:\n%s", out)
	}
}

func TestMigrateJSONFailureHasItsKeys(t *testing.T) {
	profiles, _ := migrateHome(t)
	bad := filepath.Join(profiles, "work", "projects", "-p", "a.jsonl")
	migrateWrite(t, bad, "private words")
	if !lockOut(t, bad) {
		t.Skip("permission bits do not stop this user")
	}

	_, out, _ := runMigrate(t, "--json")
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	failure := asMap(t, asMap(t, doc["profiles"].([]any)[0])["failed"].([]any)[0])
	wantKeys(t, "a failure", failure, "error", "kind", "path")
	if failure["kind"] != "file" || failure["path"] != "-p/a.jsonl" {
		t.Errorf("failure = %v", failure)
	}
}
