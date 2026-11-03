package reviews

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

// three files, each section 45 to 47 bytes. chunkBudget picks a budget above
// every single section but below any two together, so each file is its own
// chunk.
const threeFileDiff = "diff --git a/a.go b/a.go\n+alpha_line_in_a_go\n" +
	"diff --git a/b.go b/b.go\n+bravo_line_in_b_go\n" +
	"diff --git a/c.go b/c.go\n+charlie_line_in_c_go\n"

// chunkBudget is the longest file section plus one byte: it fits any one
// section but not the two shortest together (45 + 46 = 91).
func chunkBudget() int {
	longest := 0
	for _, sec := range strings.Split(threeFileDiff, "diff --git ")[1:] {
		if n := len("diff --git ") + len(sec); n > longest {
			longest = n
		}
	}
	return longest + 1
}

// chunkMarks are one unique line per file, used to tell which chunk a
// prompt carries.
var chunkMarks = []string{"+alpha_line_in_a_go", "+bravo_line_in_b_go", "+charlie_line_in_c_go"}

// chunkRig is a rig that reviews only #7 with the three file diff split
// into 3 chunks. reply picks the claude answer by the chunk mark in stdin
// (index 0, 1 or 2). the final cross-file run (stdin has "cross-file") gets
// an empty reply so chunk-only assertions keep their meaning.
func chunkRig(t *testing.T, reply func(i int) string) *rig {
	t.Helper()
	r := newRig(t)
	onlySmallPR(r)
	r.fx.rules = append([]fakeRule{{has: "pr diff 7", out: threeFileDiff}}, r.fx.rules...)
	r.svc.ChunkBytes = chunkBudget()
	r.fx.answer = func(c call) (string, bool) {
		if c.Name != "claude" {
			return "", false
		}
		if strings.Contains(c.Stdin, "cross-file") {
			return envelope("sx", "", ""), true
		}
		for i, m := range chunkMarks {
			if strings.Contains(c.Stdin, m) {
				return reply(i), true
			}
		}
		return "", false
	}
	return r
}

// envelope wraps a reviewer reply in the claude json envelope.
func envelope(sid, summary, findings string) string {
	inner := `{"summary":"` + summary + `","findings":[` + findings + `]}`
	q := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(inner)
	return `{"type":"result","session_id":"` + sid + `","result":"` + q + `"}`
}

func finding(sev, file string, line int, title string) string {
	return `{"severity":"` + sev + `","file":"` + file + `","line":` + strconv.Itoa(line) + `,"title":"` + title + `","detail":"d"}`
}

func TestBigDiffRunsClaudeOncePerChunkInOrderEachWithItsOwnDiffOnly(t *testing.T) {
	r := chunkRig(t, func(i int) string { return envelope("s", "ok", "") })
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	calls := r.claudeCalls()
	if len(calls) != 4 {
		t.Fatalf("claude ran %d times, want 4 (3 chunks and the cross-file pass)", len(calls))
	}
	if !strings.Contains(calls[3].Stdin, "cross-file") {
		t.Errorf("run 3 is not the cross-file pass: %q", calls[3].Stdin)
	}
	calls = calls[:3]
	wantNum := []string{"chunk 1 of 3", "chunk 2 of 3", "chunk 3 of 3"}
	for i, c := range calls {
		low := strings.ToLower(c.Stdin)
		if !strings.Contains(c.Stdin, wantNum[i]) {
			t.Errorf("run %d stdin lacks %q: %q", i, wantNum[i], c.Stdin)
		}
		if !strings.Contains(low, "other") || !strings.Contains(low, "separately") {
			t.Errorf("run %d stdin does not say the other files are reviewed separately: %q", i, c.Stdin)
		}
		for j, m := range chunkMarks {
			if has := strings.Contains(c.Stdin, m); has != (i == j) {
				t.Errorf("run %d: has diff of file %d = %v, want %v", i, j, has, i == j)
			}
		}
		if !strings.Contains(c.Stdin, "Fix it") || !strings.Contains(c.Stdin, "untrusted") {
			t.Errorf("run %d lacks the usual prompt text: %q", i, c.Stdin)
		}
	}
}

func TestChunkRunsUseThePickedAccountAndDistinctSessionIDs(t *testing.T) {
	r := chunkRig(t, func(i int) string { return envelope("sess-"+strconv.Itoa(i+1), "ok", "") })
	_ = r.svc.Tick(context.Background())
	seen := map[string]bool{}
	for _, c := range r.claudeCalls() {
		if len(c.Env) != 1 || c.Env[0] != "CLAUDE_CONFIG_DIR=/a/home" {
			t.Errorf("env = %v", c.Env)
		}
		id := strings.Join(c.Args, " ")
		if seen[id] {
			t.Errorf("two runs share argv %q (session id must differ)", id)
		}
		seen[id] = true
	}
	got, ok, _ := r.svc.Get("o-r-7-1111111")
	if !ok || got.Transcript != "sess-1" || got.Account != "home" {
		t.Fatalf("transcript should be the first run's session and account the pick: %+v", got)
	}
}

func TestChunkFindingsAreMergedInChunkThenReplyOrderWithFileAndLine(t *testing.T) {
	replies := []string{
		envelope("s1", "sum a", finding("high", "a.go", 3, "nil deref")+","+finding("low", "a.go", 8, "name")),
		envelope("s2", "sum b", finding("medium", "b.go", 5, "leak")),
		envelope("s3", "sum c", finding("low", "c.go", 1, "typo")+","+finding("high", "c.go", 2, "race")),
	}
	r := chunkRig(t, func(i int) string { return replies[i] })
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, ok, _ := r.svc.Get("o-r-7-1111111")
	if !ok || got.Error != "" {
		t.Fatalf("review = %+v", got)
	}
	want := []Finding{
		{Severity: "high", File: "a.go", Line: 3, Title: "nil deref"},
		{Severity: "low", File: "a.go", Line: 8, Title: "name"},
		{Severity: "medium", File: "b.go", Line: 5, Title: "leak"},
		{Severity: "low", File: "c.go", Line: 1, Title: "typo"},
		{Severity: "high", File: "c.go", Line: 2, Title: "race"},
	}
	if len(got.Findings) != len(want) {
		t.Fatalf("findings = %+v, want %d", got.Findings, len(want))
	}
	for i, w := range want {
		g := got.Findings[i]
		if g.Severity != w.Severity || g.File != w.File || g.Line != w.Line || g.Title != w.Title {
			t.Errorf("finding %d = %+v, want %+v", i, g, w)
		}
	}
}

func TestChunkSummariesAreAllKeptInChunkOrder(t *testing.T) {
	replies := []string{
		envelope("s1", "first summary", ""),
		envelope("s2", "", ""),
		envelope("s3", "third summary", ""),
	}
	r := chunkRig(t, func(i int) string { return replies[i] })
	_ = r.svc.Tick(context.Background())
	got, _, _ := r.svc.Get("o-r-7-1111111")
	a, c := strings.Index(got.Summary, "first summary"), strings.Index(got.Summary, "third summary")
	if a < 0 || c < 0 || a > c {
		t.Fatalf("summary = %q, want both texts with the first before the third", got.Summary)
	}
}

func TestADuplicateFindingFromTwoChunksAppearsOnceButSameTitleOnAnotherLineStays(t *testing.T) {
	replies := []string{
		envelope("s1", "a", finding("high", "shared.go", 4, "bad lock")),
		envelope("s2", "b", finding("high", "shared.go", 4, "bad lock")+","+finding("high", "shared.go", 40, "bad lock")),
		envelope("s3", "c", finding("low", "shared.go", 4, "bad lock")),
	}
	r := chunkRig(t, func(i int) string { return replies[i] })
	_ = r.svc.Tick(context.Background())
	got, _, _ := r.svc.Get("o-r-7-1111111")
	if len(got.Findings) != 3 {
		t.Fatalf("findings = %+v, want the line 4 high once, line 40 high, line 4 low", got.Findings)
	}
	if f := got.Findings[0]; f.Line != 4 || f.Severity != "high" {
		t.Errorf("first = %+v", f)
	}
	if f := got.Findings[1]; f.Line != 40 {
		t.Errorf("second = %+v, want the line 40 one", f)
	}
	if f := got.Findings[2]; f.Line != 4 || f.Severity != "low" {
		t.Errorf("third = %+v, want the low severity one (different severity is not a duplicate)", f)
	}
}

func TestFirstOfTwoDuplicateFindingsKeepsItsDetail(t *testing.T) {
	one := `{"severity":"high","file":"a.go","line":3,"title":"t","detail":"from chunk one"}`
	two := `{"severity":"high","file":"a.go","line":3,"title":"t","detail":"from chunk two"}`
	replies := []string{envelope("s1", "a", one), envelope("s2", "b", two), envelope("s3", "c", "")}
	r := chunkRig(t, func(i int) string { return replies[i] })
	_ = r.svc.Tick(context.Background())
	got, _, _ := r.svc.Get("o-r-7-1111111")
	if len(got.Findings) != 1 || got.Findings[0].Detail != "from chunk one" {
		t.Fatalf("findings = %+v", got.Findings)
	}
}
