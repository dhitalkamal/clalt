package activity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readAll(t *testing.T, o Options) Reply {
	t.Helper()
	r, err := Read(o)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	return r
}

// kinds lists "kind:tool" per entry, newest first.
func kinds(r Reply) []string {
	var out []string
	for _, e := range r.Entries {
		out = append(out, e.Kind+":"+e.Tool)
	}
	return out
}

func TestMainFileEveryKindNewestFirst(t *testing.T) {
	r := readAll(t, Options{Source: Source{Path: "testdata/main.jsonl"}})
	want := []string{
		"reply:", "note:", "result:Edit", "tool:Edit", "subagent-end:Agent", "subagent-start:Agent",
		"result:Bash", "tool:Bash", "result:Read", "tool:Read", "reply:", "prompt:",
	}
	got := kinds(r)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("kinds\n got %v\nwant %v", got, want)
	}
	if r.Next != "" || r.Skipped != 1 {
		t.Errorf("next %q skipped %d, want empty and 1", r.Next, r.Skipped)
	}
	if r.Entries[0].Text != "all done" || r.Entries[len(r.Entries)-1].Text != "fix the parser" {
		t.Errorf("texts %q ... %q", r.Entries[0].Text, r.Entries[len(r.Entries)-1].Text)
	}
	if r.Entries[0].T != "2026-10-27T14:00:46Z" {
		t.Errorf("t = %q", r.Entries[0].T)
	}
}

func TestToolsPairWithResultsAndGetDurations(t *testing.T) {
	r := readAll(t, Options{Source: Source{Path: "testdata/main.jsonl"}})
	by := map[string]Entry{}
	for _, e := range r.Entries {
		by[e.Kind+":"+e.Tool] = e
	}
	if e := by["tool:Read"]; e.DurationMs != 1000 || !e.OK || len(e.Files) != 1 || e.Files[0] != "src/parser.go" || e.Text != "src/parser.go" {
		t.Errorf("read tool = %+v", e)
	}
	if e := by["result:Read"]; e.DurationMs != 1000 || !e.OK || e.Text != "package parser" {
		t.Errorf("read result = %+v", e)
	}
	if e := by["tool:Bash"]; e.DurationMs != 3000 || e.OK || e.Text != "go test ./..." {
		t.Errorf("bash tool = %+v", e)
	}
	if e := by["result:Bash"]; e.OK || e.Text != "FAIL parser" || e.DurationMs != 3000 {
		t.Errorf("bash result = %+v", e)
	}
	if e := by["tool:Edit"]; len(e.Files) != 1 || e.Files[0] != "src/parser.go" {
		t.Errorf("edit files = %v", e.Files)
	}
}

func TestSubagentMarkersCarryTheAgentId(t *testing.T) {
	r := readAll(t, Options{Source: Source{Path: "testdata/main.jsonl"}})
	var start, end Entry
	for _, e := range r.Entries {
		switch e.Kind {
		case "subagent-start":
			start = e
		case "subagent-end":
			end = e
		}
	}
	if start.Agent != "a111" || end.Agent != "a111" || start.Text != "survey the repo" || end.Text != "survey done" || end.DurationMs != 30000 || !end.OK {
		t.Errorf("start %+v end %+v", start, end)
	}
}

func TestSidechainFileGivesTheRunOfOneAgent(t *testing.T) {
	r := readAll(t, Options{Source: Source{Path: "testdata/agent-a111.jsonl", Agent: "a111"}})
	want := []string{"reply:", "result:Grep", "tool:Grep", "prompt:"}
	if got := kinds(r); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("kinds %v, want %v", got, want)
	}
	for _, e := range r.Entries {
		if e.Agent != "a111" {
			t.Errorf("entry %s has agent %q", e.Kind, e.Agent)
		}
	}
	if r.Entries[2].DurationMs != 2500 || len(r.Entries[2].Files) != 1 || r.Entries[2].Files[0] != "src" {
		t.Errorf("grep = %+v", r.Entries[2])
	}
}

func TestLegacySidechainLinesInTheMainFile(t *testing.T) {
	main := readAll(t, Options{Source: Source{Path: "testdata/legacy.jsonl"}})
	for _, e := range main.Entries {
		if e.Text == "inner prompt" || e.Text == "inner reply" {
			t.Errorf("sidechain line leaked into the main view: %+v", e)
		}
	}
	sub := readAll(t, Options{Source: Source{Path: "testdata/legacy.jsonl", Agent: "b222", Legacy: true}})
	if got := kinds(sub); strings.Join(got, " ") != "reply: prompt:" {
		t.Errorf("legacy run = %v", got)
	}
	if sub.Entries[0].Agent != "b222" {
		t.Errorf("agent = %q", sub.Entries[0].Agent)
	}
}

func TestPagingWithCursors(t *testing.T) {
	var all []Entry
	cursor := ""
	pages := 0
	for {
		r := readAll(t, Options{Source: Source{Path: "testdata/main.jsonl"}, Limit: 4, Before: cursor})
		if len(r.Entries) == 0 || len(r.Entries) > 4 {
			t.Fatalf("page %d has %d entries", pages, len(r.Entries))
		}
		all = append(all, r.Entries...)
		pages++
		if r.Next == "" {
			break
		}
		cursor = r.Next
		if pages > 10 {
			t.Fatal("paging does not end")
		}
	}
	whole := readAll(t, Options{Source: Source{Path: "testdata/main.jsonl"}})
	a, _ := json.Marshal(all)
	b, _ := json.Marshal(whole.Entries)
	if string(a) != string(b) || pages < 3 {
		t.Errorf("pages (%d) differ from the whole read\n%s\n%s", pages, a, b)
	}
}

func TestPagedToolStillGetsItsResultFromANewerPage(t *testing.T) {
	// limit 1 puts every result alone on a page before its tool call
	cursor := ""
	for i := 0; i < 20; i++ {
		r := readAll(t, Options{Source: Source{Path: "testdata/main.jsonl"}, Limit: 1, Before: cursor})
		for _, e := range r.Entries {
			if e.Kind == "result" && e.Tool == "" {
				t.Errorf("result without its tool name: %+v", e)
			}
			if e.Kind == "tool" && e.Tool == "Bash" && e.DurationMs != 3000 {
				t.Errorf("bash duration on a page of its own = %d", e.DurationMs)
			}
		}
		if cursor = r.Next; cursor == "" {
			return
		}
	}
}

func TestBadCursorIsAnError(t *testing.T) {
	for _, c := range []string{"nope", "!!!", "djE6LTU", "djE6OTk5OTk5OTk5"} {
		if _, err := Read(Options{Source: Source{Path: "testdata/main.jsonl"}, Before: c}); err != ErrBadCursor {
			t.Errorf("cursor %q: err = %v, want ErrBadCursor", c, err)
		}
	}
}

func TestMissingFile(t *testing.T) {
	if _, err := Read(Options{Source: Source{Path: "testdata/none.jsonl"}}); !os.IsNotExist(err) {
		t.Errorf("err = %v, want not exist", err)
	}
}

func TestLimitDefaultsAndIsCapped(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "many.jsonl")
	var b strings.Builder
	for i := 0; i < 400; i++ {
		b.WriteString(`{"type":"user","timestamp":"2026-10-27T14:00:00Z","message":{"role":"user","content":"p"}}` + "\n")
	}
	if err := os.WriteFile(p, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if n := len(readAll(t, Options{Source: Source{Path: p}}).Entries); n != 100 {
		t.Errorf("default limit gave %d", n)
	}
	if n := len(readAll(t, Options{Source: Source{Path: p}, Limit: 9999}).Entries); n != 300 {
		t.Errorf("max limit gave %d", n)
	}
}

func TestScanBoundStopsAndLeavesACursor(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "long.jsonl")
	line := `{"type":"user","timestamp":"2026-10-27T14:00:00Z","message":{"role":"user","content":"` + strings.Repeat("x", 200) + `"}}` + "\n"
	if err := os.WriteFile(p, []byte(strings.Repeat(line, 100)), 0o600); err != nil {
		t.Fatal(err)
	}
	r := readAll(t, Options{Source: Source{Path: p}, Limit: 300, MaxScan: 3000})
	if n := len(r.Entries); n == 0 || n >= 100 {
		t.Fatalf("bounded read gave %d entries", n)
	}
	if r.Next == "" {
		t.Fatal("a cut read must leave a cursor")
	}
	more := readAll(t, Options{Source: Source{Path: p}, Limit: 300, MaxScan: 3000, Before: r.Next})
	if len(more.Entries) == 0 {
		t.Error("the cursor does not lead to older entries")
	}
}

func TestOneLineLongerThanTheBoundDoesNotLoop(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "huge.jsonl")
	huge := `{"type":"user","timestamp":"2026-10-27T14:00:00Z","message":{"role":"user","content":"` + strings.Repeat("y", 5000) + `"}}` + "\n"
	if err := os.WriteFile(p, []byte(huge), 0o600); err != nil {
		t.Fatal(err)
	}
	r := readAll(t, Options{Source: Source{Path: p}, MaxScan: 1000})
	if len(r.Entries) != 0 || r.Next != "" {
		t.Errorf("got %d entries, next %q", len(r.Entries), r.Next)
	}
}

func TestOversizedLinesAreSkippedAndCounted(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "big.jsonl")
	ok := `{"type":"user","timestamp":"2026-10-27T14:00:00Z","message":{"role":"user","content":"fine"}}` + "\n"
	big := `{"type":"user","message":{"role":"user","content":"` + strings.Repeat("z", 3000) + `"}}` + "\n"
	if err := os.WriteFile(p, []byte(ok+big+ok), 0o600); err != nil {
		t.Fatal(err)
	}
	r := readAll(t, Options{Source: Source{Path: p}, MaxLine: 1000})
	if len(r.Entries) != 2 || r.Skipped != 1 {
		t.Errorf("entries %d skipped %d, want 2 and 1", len(r.Entries), r.Skipped)
	}
}

func TestFieldCapAndWholeReplyCap(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "caps.jsonl")
	long := strings.Repeat("a", maxText+500)
	line := `{"type":"assistant","timestamp":"2026-10-27T14:00:00Z","message":{"role":"assistant","content":[{"type":"text","text":"` + long + `"}]}}` + "\n"
	if err := os.WriteFile(p, []byte(strings.Repeat(line, 150)), 0o600); err != nil {
		t.Fatal(err)
	}
	r := readAll(t, Options{Source: Source{Path: p}, Limit: 300})
	first := r.Entries[0].Text
	if !strings.HasSuffix(first, cutMarker) || len([]rune(first)) != maxText+len([]rune(cutMarker)) {
		t.Errorf("text is %d runes, ends %q", len([]rune(first)), first[len(first)-10:])
	}
	raw, _ := json.Marshal(r)
	if len(raw) > maxReply+maxText*2 {
		t.Errorf("reply is %d bytes", len(raw))
	}
	if r.Next == "" || len(r.Entries) >= 150 {
		t.Errorf("a capped reply must leave a cursor: %d entries, next %q", len(r.Entries), r.Next)
	}
}

func TestCleanStripsControlAndFormatKeepsNewlineAndTab(t *testing.T) {
	in := "a\x1b[31mred\x1b[0m\x00b\u202ec\u200bd\re\nf\tg\xffh"
	if got := cleanText(in, 100); got != "aredbcde\nf\tgh" {
		t.Errorf("cleanText = %q", got)
	}
}

func TestEmptyListsEncodeAsArrays(t *testing.T) {
	r := readAll(t, Options{Source: Source{Path: "testdata/main.jsonl"}, Limit: 1})
	raw, _ := json.Marshal(r.Entries[0])
	if !strings.Contains(string(raw), `"files":[]`) {
		t.Errorf("entry json = %s", raw)
	}
}
