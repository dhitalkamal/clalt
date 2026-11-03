package office

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
)

func toolLine(at, id, name, input string) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":"%s","message":{"role":"assistant","content":[{"type":"tool_use","id":"%s","name":"%s","input":%s}]}}`+"\n", at, id, name, input)
}

func resultLine(at, id string, isErr bool) string {
	return fmt.Sprintf(`{"type":"user","timestamp":"%s","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"%s","is_error":%t,"content":"SECRET OUTPUT"}]}}`+"\n", at, id, isErr)
}

func appendTo(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}

// doingFixture is a live run session with a transcript, built at dnow.
func doingFixture(t *testing.T) (Real, string) {
	t.Helper()
	r, main := realFixture(t, "/Users/x/p")
	err := r.Registry.Create(registry.Record{Handle: runHandle, Account: "work", Cwd: "/Users/x/p", ClaudeSessionID: sessionUUID,
		WrapperPID: os.Getpid(), PID: os.Getpid(), StartedAt: dnow.Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	r.Accounts = staticAccounts{{ID: "work", Label: "work"}}
	r.Now = func() time.Time { return dnow }
	return r, main
}

func TestDoingOfASessionFromItsTranscriptTail(t *testing.T) {
	r, main := doingFixture(t)
	appendTo(t, main, toolLine("2026-10-31T10:00:10Z", "t1", "Bash", `{"command":"echo SECRET"}`))
	d := r.Doing(runHandle, "")
	if d == nil || d.Kind != "run" || d.State != "running" || d.Tool != "Bash" {
		t.Fatalf("running: %+v", d)
	}
	appendTo(t, main, resultLine("2026-10-31T10:00:14Z", "t1", true))
	if d = r.Doing(runHandle, ""); d == nil || d.State != "fail" {
		t.Fatalf("fail: %+v", d)
	}
	appendTo(t, main, toolLine("2026-10-31T10:00:16Z", "t2", "Bash", `{"command":"go test"}`)+resultLine("2026-10-31T10:00:20Z", "t2", false))
	if d = r.Doing(runHandle, ""); d == nil || d.State != "ok" {
		t.Fatalf("ok: %+v", d)
	}
}

func TestDoingIsNilWhenTheTranscriptIsMissingOrStale(t *testing.T) {
	r, main := doingFixture(t)
	appendTo(t, main, toolLine("2026-10-31T09:50:00Z", "t1", "Read", `{"file_path":"a"}`))
	if d := r.Doing(runHandle, ""); d != nil {
		t.Errorf("stale: %+v", d)
	}
	if d := r.Doing("ffffffffffffffff", ""); d != nil {
		t.Errorf("unknown handle: %+v", d)
	}
}

func TestDoingKeepsNoNamesCommandsOrText(t *testing.T) {
	r, main := doingFixture(t)
	appendTo(t, main, toolLine("2026-10-31T10:00:10Z", "t1", "Bash", `{"command":"cat /etc/SECRET"}`)+resultLine("2026-10-31T10:00:12Z", "t1", false))
	snap := doingSnapshot(t, r)
	if strings.Contains(snap, "SECRET") || strings.Contains(snap, "/etc") {
		t.Errorf("leak in %s", snap)
	}
}

func TestDoingIsCachedBySizeAndMtime(t *testing.T) {
	r, main := doingFixture(t)
	appendTo(t, main, toolLine("2026-10-31T10:00:10Z", "t1", "Read", `{"file_path":"a"}`))
	before := doingReads.Load()
	r.Doing(runHandle, "")
	r.Doing(runHandle, "")
	r.Doing(runHandle, "")
	if n := doingReads.Load() - before; n != 1 {
		t.Errorf("reads = %d, want 1", n)
	}
	appendTo(t, main, resultLine("2026-10-31T10:00:11Z", "t1", false))
	r.Doing(runHandle, "")
	if n := doingReads.Load() - before; n != 2 {
		t.Errorf("reads after change = %d, want 2", n)
	}
}

func TestDoingStalenessIsJudgedOnEveryCallNotCached(t *testing.T) {
	r, main := doingFixture(t)
	appendTo(t, main, toolLine("2026-10-31T10:00:10Z", "t1", "Read", `{"file_path":"a"}`))
	if r.Doing(runHandle, "") == nil {
		t.Fatal("fresh should have doing")
	}
	r.Now = func() time.Time { return dnow.Add(5 * time.Minute) }
	if d := r.Doing(runHandle, ""); d != nil {
		t.Errorf("should have gone stale: %+v", d)
	}
}

func TestSubagentDoingReadsItsOwnFile(t *testing.T) {
	r, main := doingFixture(t)
	dir := filepath.Join(strings.TrimSuffix(main, ".jsonl"), "subagents")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "agent-a1.jsonl")
	if err := os.WriteFile(sub, []byte(toolLine("2026-10-31T10:00:20Z", "s1", "WebSearch", `{"query":"x"}`)), 0o600); err != nil {
		t.Fatal(err)
	}
	if d := r.Doing(runHandle, "a1"); d == nil || d.Kind != "web" {
		t.Errorf("subagent doing %+v", d)
	}
}

func TestBuildFillsDoingForLiveSessionsAndRunningSubagents(t *testing.T) {
	r, main := doingFixture(t)
	appendTo(t, main, toolLine("2026-10-31T10:00:10Z", "t1", "Edit", `{"file_path":"a"}`))
	sub := filepath.Join(strings.TrimSuffix(main, ".jsonl"), "subagents")
	_ = os.MkdirAll(sub, 0o700)
	_ = os.WriteFile(filepath.Join(sub, "agent-a1.jsonl"), []byte(toolLine("2026-10-31T10:00:20Z", "s1", "Grep", `{"pattern":"x"}`)), 0o600)
	r.Feed.UpdateSession(runHandle, "work", dnow, func(s *feed.SessionState) bool {
		s.State = feed.StateBusy
		s.Subagents = []feed.Subagent{{ID: "a1", Type: "Explore", State: "running", StartedAt: dnow}, {ID: "a2", Type: "Explore", State: "done", StartedAt: dnow}}
		return true
	})
	snap, err := Build(RealSources(r))
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Sessions) != 1 {
		t.Fatalf("sessions %d", len(snap.Sessions))
	}
	s := snap.Sessions[0]
	if s.Doing == nil || s.Doing.Kind != "edit" {
		t.Errorf("session doing %+v", s.Doing)
	}
	if len(s.Subagents) != 2 || s.Subagents[0].Doing == nil || s.Subagents[0].Doing.Kind != "read" || s.Subagents[1].Doing != nil {
		t.Errorf("subagents %+v", s.Subagents)
	}
}

// doingSnapshot is the snapshot of r as the page gets it.
func doingSnapshot(t *testing.T, r Real) string {
	t.Helper()
	snap, err := Build(RealSources(r))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
