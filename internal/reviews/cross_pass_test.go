package reviews

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

// isCrossCall tells the final cross-file run apart from the chunk runs: no
// "chunk N of M" text and the words "cross-file".
func isCrossCall(c call) bool {
	return strings.Contains(c.Stdin, "cross-file") && !strings.Contains(c.Stdin, " of 3")
}

// crossRig is a chunkRig whose cross-file run (no chunk mark in stdin) is
// answered with cross.
func crossRig(t *testing.T, reply func(i int) string, cross string) *rig {
	t.Helper()
	r := chunkRig(t, reply)
	chunkAnswer := r.fx.answer
	r.fx.answer = func(c call) (string, bool) {
		if c.Name == "claude" && isCrossCall(c) {
			return cross, true
		}
		return chunkAnswer(c)
	}
	return r
}

func plainReplies(i int) string {
	return envelope("sess-"+strconv.Itoa(i+1), "sum "+strconv.Itoa(i+1), "")
}

func TestThreeChunksRunClaudeFourTimesTheLastBeingTheCrossFilePass(t *testing.T) {
	r := crossRig(t, plainReplies, envelope("sx", "x", ""))
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	calls := r.claudeCalls()
	if len(calls) != 4 {
		t.Fatalf("claude ran %d times, want 4 (3 chunks and one cross-file pass)", len(calls))
	}
	for i, c := range calls[:3] {
		if strings.Contains(c.Stdin, "cross-file") {
			t.Errorf("chunk run %d is a cross-file prompt: %q", i, c.Stdin)
		}
	}
	if !isCrossCall(calls[3]) {
		t.Fatalf("last run is not the cross-file pass: %q", calls[3].Stdin)
	}
}

func TestCrossPassPromptHasSummariesAndFilesButNoDiff(t *testing.T) {
	replies := []string{envelope("s1", "alpha summary", ""), envelope("s2", "", ""), envelope("s3", "gamma summary", "")}
	r := crossRig(t, func(i int) string { return replies[i] }, envelope("sx", "", ""))
	_ = r.svc.Tick(context.Background())
	calls := r.claudeCalls()
	if len(calls) != 4 {
		t.Fatalf("claude ran %d times, want 4", len(calls))
	}
	p := calls[3].Stdin
	for _, want := range []string{"alpha summary", "gamma summary", "a.go", "b.go", "c.go", "cross-file", "JSON"} {
		if !strings.Contains(p, want) {
			t.Errorf("cross prompt lacks %q: %q", want, p)
		}
	}
	for _, line := range strings.Split(threeFileDiff, "\n") {
		if line != "" && strings.Contains(p, line) {
			t.Errorf("cross prompt carries diff line %q: %q", line, p)
		}
	}
}

func TestCrossPassRunsInTheSameCheckoutAsTheChunkRuns(t *testing.T) {
	r := crossRig(t, plainReplies, envelope("sx", "", ""))
	_ = r.svc.Tick(context.Background())
	calls := r.claudeCalls()
	if len(calls) != 4 {
		t.Fatalf("claude ran %d times, want 4", len(calls))
	}
	if calls[0].Dir == "" {
		t.Fatalf("chunk run has no dir")
	}
	if calls[3].Dir != calls[0].Dir {
		t.Errorf("cross dir = %q, want the chunk dir %q", calls[3].Dir, calls[0].Dir)
	}
}

func TestCrossFindingsComeAfterChunkFindingsAndDedupAgainstThem(t *testing.T) {
	replies := []string{
		envelope("s1", "a", finding("high", "a.go", 3, "nil deref")),
		envelope("s2", "b", ""),
		envelope("s3", "c", finding("low", "c.go", 1, "typo")),
	}
	cross := envelope("sx", "", finding("high", "a.go", 3, "nil deref")+","+finding("medium", "b.go", 9, "api mismatch"))
	r := crossRig(t, func(i int) string { return replies[i] }, cross)
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, ok, _ := r.svc.Get("o-r-7-1111111")
	if !ok || got.Error != "" {
		t.Fatalf("review = %+v", got)
	}
	want := []Finding{
		{Severity: "high", File: "a.go", Line: 3, Title: "nil deref"},
		{Severity: "low", File: "c.go", Line: 1, Title: "typo"},
		{Severity: "medium", File: "b.go", Line: 9, Title: "api mismatch"},
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

func TestCrossSummaryIsAppendedAfterTheChunkSummaries(t *testing.T) {
	r := crossRig(t, plainReplies, envelope("sx", "cross wrap up", ""))
	_ = r.svc.Tick(context.Background())
	got, _, _ := r.svc.Get("o-r-7-1111111")
	last3 := strings.Index(got.Summary, "sum 3")
	x := strings.Index(got.Summary, "cross wrap up")
	if last3 < 0 || x < 0 || x < last3 {
		t.Fatalf("summary = %q, want the cross summary after the chunk summaries", got.Summary)
	}
}

func TestAnEmptyCrossSummaryAddsNothingToTheSummary(t *testing.T) {
	r := crossRig(t, plainReplies, envelope("sx", "", ""))
	_ = r.svc.Tick(context.Background())
	got, _, _ := r.svc.Get("o-r-7-1111111")
	for _, want := range []string{"sum 1", "sum 2", "sum 3"} {
		if !strings.Contains(got.Summary, want) {
			t.Errorf("summary = %q, lacks %q", got.Summary, want)
		}
	}
	if len(r.claudeCalls()) != 4 {
		t.Fatalf("claude ran %d times, want 4", len(r.claudeCalls()))
	}
}

func TestTranscriptStaysTheFirstRunsSessionWithACrossPass(t *testing.T) {
	r := crossRig(t, plainReplies, envelope("sess-cross", "x", ""))
	_ = r.svc.Tick(context.Background())
	if len(r.claudeCalls()) != 4 {
		t.Fatalf("claude ran %d times, want 4", len(r.claudeCalls()))
	}
	got, ok, _ := r.svc.Get("o-r-7-1111111")
	if !ok || got.Transcript != "sess-1" {
		t.Fatalf("transcript = %q, want the first run's sess-1", got.Transcript)
	}
}

func TestASingleChunkReviewHasNoCrossPass(t *testing.T) {
	r := newRig(t)
	onlySmallPR(r)
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	calls := r.claudeCalls()
	if len(calls) != 1 {
		t.Fatalf("claude ran %d times, want exactly 1", len(calls))
	}
	if strings.Contains(calls[0].Stdin, "cross-file") {
		t.Errorf("single run is a cross-file prompt: %q", calls[0].Stdin)
	}
}
