package activity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLines(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "t.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAsyncLaunchResultIsDroppedAndStartKeepsTheId(t *testing.T) {
	p := writeLines(t,
		`{"type":"assistant","timestamp":"2026-10-27T14:00:00Z","message":{"content":[{"type":"tool_use","id":"u1","name":"Agent","input":{"description":"bg job"}}]}}`,
		`{"type":"user","timestamp":"2026-10-27T14:00:01Z","toolUseResult":{"isAsync":true,"status":"async_launched","agentId":"a777"},"message":{"content":[{"type":"tool_result","tool_use_id":"u1","content":"launched"}]}}`,
	)
	r := readAll(t, Options{Source: Source{Path: p}})
	if len(r.Entries) != 1 || r.Entries[0].Kind != KindSubagentStart || r.Entries[0].Agent != "a777" || r.Entries[0].DurationMs != 0 {
		t.Errorf("entries = %+v", r.Entries)
	}
}

func TestResultWithoutItsCallStaysAResult(t *testing.T) {
	p := writeLines(t, `{"type":"user","timestamp":"2026-10-27T14:00:01Z","message":{"content":[{"type":"tool_result","tool_use_id":"gone","content":"x"}]}}`)
	r := readAll(t, Options{Source: Source{Path: p}})
	if len(r.Entries) != 1 || r.Entries[0].Kind != KindResult || r.Entries[0].Tool != "" {
		t.Errorf("entries = %+v", r.Entries)
	}
}

func TestFileListsAreDedupedAndCapped(t *testing.T) {
	var names []string
	for i := 0; i < 40; i++ {
		names = append(names, `"f`+string(rune('a'+i%26))+string(rune('a'+i/26))+`"`)
	}
	p := writeLines(t, `{"type":"assistant","timestamp":"2026-10-27T14:00:00Z","message":{"content":[{"type":"tool_use","id":"u1","name":"Glob","input":{"path":"fa","paths":[`+strings.Join(names, ",")+`]}}]}}`)
	r := readAll(t, Options{Source: Source{Path: p}})
	if n := len(r.Entries[0].Files); n != maxFiles {
		t.Errorf("files = %d, want %d", n, maxFiles)
	}
}

func TestBackReaderLinesAcrossChunksAndATrailingLineWithoutNewline(t *testing.T) {
	var lines []string
	for i := 0; i < 5; i++ {
		lines = append(lines, strings.Repeat(string(rune('a'+i)), chunk/2+7))
	}
	p := filepath.Join(t.TempDir(), "x")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Open(p)
	defer f.Close()
	fi, _ := f.Stat()
	br := newBackReader(f, fi.Size(), 1<<30, 1<<30)
	for i := 4; i >= 0; i-- {
		line, _, ok := br.next()
		if !ok || string(line) != lines[i] {
			t.Fatalf("line %d: ok %v len %d", i, ok, len(line))
		}
	}
	if _, _, ok := br.next(); ok {
		t.Error("more lines than written")
	}
}

func TestSystemErrorsAndOtherTypes(t *testing.T) {
	p := writeLines(t,
		`{"type":"system","level":"error","timestamp":"2026-10-27T14:00:00Z","content":"api failed"}`,
		`{"type":"system","subtype":"stop_hook_summary","timestamp":"2026-10-27T14:00:01Z"}`,
		`{"type":"queue-operation","timestamp":"2026-10-27T14:00:02Z","content":"x"}`,
		`[1,2]`,
	)
	r := readAll(t, Options{Source: Source{Path: p}})
	if len(r.Entries) != 1 || r.Entries[0].Kind != KindNote || r.Entries[0].Text != "api failed" || r.Skipped != 1 {
		t.Errorf("entries %+v skipped %d", r.Entries, r.Skipped)
	}
}
