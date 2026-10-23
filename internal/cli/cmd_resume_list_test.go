package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

func convID(n int) string { return fmt.Sprintf("aaaaaaaa-0000-0000-0000-%012d", n) }

// seedConversation writes a synthetic transcript into the shared projects
// folder: one prompt and one reply, last used at ts.
func (e *testEnv) seedConversation(id, cwd, prompt, ts string) string {
	e.t.Helper()
	dir := filepath.Join(config.ClaudeProjectsDir(), "-proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		e.t.Fatal(err)
	}
	user, _ := json.Marshal(map[string]any{
		"type": "user", "timestamp": ts, "cwd": cwd, "message": map[string]any{"content": prompt},
	})
	reply, _ := json.Marshal(map[string]any{
		"type": "assistant", "timestamp": ts, "cwd": cwd, "message": map[string]any{"model": "claude-opus-5-5"},
	})
	path := filepath.Join(dir, id+".jsonl")
	if err := os.WriteFile(path, []byte(string(user)+"\n"+string(reply)+"\n"), 0o644); err != nil {
		e.t.Fatal(err)
	}
	return path
}

func TestResumeListsConversationsNewestFirst(t *testing.T) {
	e := newEnv(t)
	e.useFakeDeps()
	e.at(theClock)
	e.seedConversation(convID(1), "/w/old", "the older one", "2026-10-20T10:00:00Z")
	e.seedConversation(convID(2), "/w/new", "the newer one", "2026-10-22T11:00:00Z")

	if code := e.run("resume"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	out := e.out.String()
	newer, older := strings.Index(out, "the newer one"), strings.Index(out, "the older one")
	if newer < 0 || older < 0 || newer > older {
		t.Errorf("order wrong:\n%s", out)
	}
	for _, want := range []string{"aaaaaaaa", "opus-5-5", "/w/new", "1h ago", "2d ago"} {
		if !strings.Contains(out, want) {
			t.Errorf("the list is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, convID(2)) {
		t.Errorf("the list shows whole ids, not 8 characters:\n%s", out)
	}
}

func TestResumeJSONCarriesEveryField(t *testing.T) {
	e := newEnv(t)
	e.useFakeDeps()
	e.seedConversation(convID(1), "/w/app", "hello", "2026-10-22T11:00:00Z")

	if code := e.run("resume", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	var rows []map[string]any
	if err := json.Unmarshal(e.out.Bytes(), &rows); err != nil || len(rows) != 1 {
		t.Fatalf("%v: %s", err, e.out.String())
	}
	r := rows[0]
	for key, want := range map[string]any{
		"id": convID(1), "title": "hello", "cwd": "/w/app", "model": "claude-opus-5-5",
		"lastActive": "2026-10-22T11:00:00Z", "project": "-proj",
	} {
		if r[key] != want {
			t.Errorf("%s = %v, want %v", key, r[key], want)
		}
	}
	if size, _ := r["size"].(float64); size <= 0 {
		t.Errorf("size = %v", r["size"])
	}
	if _, ok := r["entrypoint"]; !ok {
		t.Error("entrypoint is missing")
	}
}

func TestResumeFilterAndLimit(t *testing.T) {
	e := newEnv(t)
	e.useFakeDeps()
	e.seedConversation(convID(1), "/w/a", "fix login", "2026-10-20T10:00:00Z")
	e.seedConversation(convID(2), "/w/b", "write docs", "2026-10-21T10:00:00Z")
	e.seedConversation(convID(3), "/w/c", "more docs", "2026-10-22T10:00:00Z")

	e.run("resume", "docs", "--json")
	var rows []map[string]any
	json.Unmarshal(e.out.Bytes(), &rows)
	if len(rows) != 2 {
		t.Errorf("filter docs gave %d rows", len(rows))
	}

	e.reset()
	e.run("resume", "--limit", "1", "--json")
	rows = nil
	json.Unmarshal(e.out.Bytes(), &rows)
	if len(rows) != 1 {
		t.Errorf("limit 1 gave %d rows", len(rows))
	}

	e.reset()
	e.run("resume", "--limit=0", "--json")
	rows = nil
	json.Unmarshal(e.out.Bytes(), &rows)
	if len(rows) != 3 {
		t.Errorf("limit 0 gave %d rows, want all", len(rows))
	}
}

func TestResumeListDefaultsToTwentyRows(t *testing.T) {
	e := newEnv(t)
	e.useFakeDeps()
	for i := 1; i <= 25; i++ {
		e.seedConversation(convID(i), "/w/a", "p", fmt.Sprintf("2026-10-%02dT10:00:00Z", i))
	}

	e.run("resume", "--json")

	var rows []map[string]any
	json.Unmarshal(e.out.Bytes(), &rows)
	if len(rows) != 20 {
		t.Errorf("%d rows, want 20", len(rows))
	}
}

func TestResumeWithNoConversationsSaysSo(t *testing.T) {
	e := newEnv(t)
	e.useFakeDeps()

	if code := e.run("resume"); code != 0 || !strings.Contains(e.out.String(), "no conversations") {
		t.Errorf("exit %d: %q", code, e.out.String())
	}
	e.reset()
	e.run("resume", "--json")
	if strings.TrimSpace(e.out.String()) != "[]" {
		t.Errorf("json = %q, want an empty array", e.out.String())
	}
}

func TestResumeListCleansWhatItReadsFromDisk(t *testing.T) {
	e := newEnv(t)
	e.useFakeDeps()
	e.seedConversation(convID(1), "/w/\x1b]0;evil\x07app", "\x1b[2Jboom", "2026-10-22T11:00:00Z")

	e.run("resume")

	if hasControl(e.out.String()) {
		t.Errorf("control characters reached the terminal: %q", e.out.String())
	}
}

func TestResumeListMakesNoState(t *testing.T) {
	e := newEnv(t)
	e.useFakeDeps()

	e.run("resume")

	if exists(config.ClaltDir()) {
		t.Errorf("resume created %s", config.ClaltDir())
	}
}

func TestResumeUsageErrors(t *testing.T) {
	cases := [][]string{
		{"resume", "--account", "work"},
		{"resume", "--limit", "x"},
		{"resume", "--limit", "-1"},
		{"resume", "--nope"},
		{"resume", "--account"},
		{"resume", "aaaaaaaa", "--limit", "3"},
	}
	for _, args := range cases {
		e := newEnv(t)
		e.useFakeDeps()
		if code := e.run(args...); code != 2 || !strings.Contains(e.errOut.String(), "usage:") {
			t.Errorf("%v: exit %d: %s", args, code, e.errOut.String())
		}
	}
}
