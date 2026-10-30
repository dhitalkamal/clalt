package reviews

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
)

// chunkNo is the chunk number a claude prompt is for (1 to 3), 0 when it is
// not a chunk prompt.
func chunkNo(c call) int {
	for n := 1; n <= 3; n++ {
		if strings.Contains(c.Stdin, "chunk "+strconv.Itoa(n)+" of 3") {
			return n
		}
	}
	return 0
}

// failChunks makes the claude run of each listed chunk fail with its reason.
func failChunks(r *rig, reasons map[int]string) {
	r.fx.fail = func(c call) error {
		if c.Name != "claude" || chunkNo(c) == 0 {
			return nil
		}
		if why, ok := reasons[chunkNo(c)]; ok {
			return errors.New(why)
		}
		return nil
	}
}

// sumReplies gives each chunk a finding in its own file and a summary.
func sumReplies(i int) string {
	f := string(rune('a'+i)) + ".go"
	return envelope("sess-"+strconv.Itoa(i+1), "sum "+strconv.Itoa(i+1), finding("high", f, i+1, "bug in "+f))
}

func TestAFailedMiddleChunkStillMergesTheOtherChunksAndNamesItInTheError(t *testing.T) {
	r := crossRig(t, sumReplies, envelope("sx", "x", ""))
	failChunks(r, map[int]string{2: "claude exited 1: boom"})
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatalf("tick = %v, want nil (the review is saved)", err)
	}
	got, ok, _ := r.svc.Get("o-r-7-1111111")
	if !ok {
		t.Fatal("review not saved")
	}
	want := "partial review: chunk 2 of 3 failed (files: b.go): claude exited 1: boom"
	if got.Error != want {
		t.Errorf("error = %q, want %q", got.Error, want)
	}
	if got.Skipped != "" {
		t.Errorf("skipped = %q, want empty so the UI shows the error", got.Skipped)
	}
	if len(got.Findings) != 2 || got.Findings[0].File != "a.go" || got.Findings[1].File != "c.go" {
		t.Errorf("findings = %+v, want the a.go and c.go ones", got.Findings)
	}
	for _, w := range []string{"sum 1", "sum 3"} {
		if !strings.Contains(got.Summary, w) {
			t.Errorf("summary = %q, lacks %q", got.Summary, w)
		}
	}
	if strings.Contains(got.Summary, "sum 2") {
		t.Errorf("summary = %q has the failed chunk's text", got.Summary)
	}
}

func TestCrossPassDoesNotRunWhenAChunkFailed(t *testing.T) {
	r := crossRig(t, sumReplies, envelope("sx", "x", ""))
	failChunks(r, map[int]string{2: "claude exited 1: boom"})
	_ = r.svc.Tick(context.Background())
	calls := r.claudeCalls()
	if len(calls) != 3 {
		t.Fatalf("claude ran %d times, want 3 (the three chunks, no cross-file pass)", len(calls))
	}
	for i, c := range calls {
		if isCrossCall(c) {
			t.Errorf("run %d is a cross-file prompt", i)
		}
	}
}

// fourFileDiff packs into 3 chunks at budget 100: a.go and b.go together,
// then c.go, then d.go.
const fourFileDiff = "diff --git a/a.go b/a.go\n+a1\n" +
	"diff --git a/b.go b/b.go\n+b1\n" +
	"diff --git a/c.go b/c.go\n+c_line_that_is_long_enough_to_fill_a_chunk_by_itself_ok\n" +
	"diff --git a/d.go b/d.go\n+d_line_that_is_long_enough_to_fill_a_chunk_by_itself_ok\n"

func TestAFailedChunkOfSeveralFilesListsThemCommaSeparatedInDiffOrder(t *testing.T) {
	r := crossRig(t, sumReplies, envelope("sx", "x", ""))
	r.fx.rules = append([]fakeRule{{has: "pr diff 7", out: fourFileDiff}}, r.fx.rules...)
	r.svc.ChunkBytes = 100
	if n := len(splitDiff(fourFileDiff, 100)); n != 3 {
		t.Fatalf("rig splits into %d chunks, want 3", n)
	}
	r.fx.answer = func(c call) (string, bool) {
		if c.Name == "claude" {
			return envelope("s", "ok", ""), true
		}
		return "", false
	}
	failChunks(r, map[int]string{1: "claude exited 1: boom"})
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _, _ := r.svc.Get("o-r-7-1111111")
	want := "partial review: chunk 1 of 3 failed (files: a.go, b.go): claude exited 1: boom"
	if got.Error != want {
		t.Errorf("error = %q, want %q", got.Error, want)
	}
}

func TestTwoFailedChunksAreBothNamedInChunkOrderAndTheMiddleOneIsKept(t *testing.T) {
	r := crossRig(t, sumReplies, envelope("sx", "x", ""))
	why1 := "claude exited 1: " + strings.Repeat("rate limit reached for this account, ", 5) + "rate limit reached for this account"
	why3 := "claude exited 2: " + strings.Repeat("network went away mid reply, ", 7) + "network went away mid reply"
	failChunks(r, map[int]string{1: why1, 3: why3})
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, ok, _ := r.svc.Get("o-r-7-1111111")
	if !ok {
		t.Fatal("review not saved")
	}
	want := "partial review: chunk 1 of 3 failed (files: a.go): " + why1 +
		"; chunk 3 of 3 failed (files: c.go): " + why3
	if got.Error != want {
		t.Errorf("error = %q, want %q", got.Error, want)
	}
	if len(got.Findings) != 1 || got.Findings[0].File != "b.go" {
		t.Errorf("findings = %+v, want only the b.go one", got.Findings)
	}
	if len(r.claudeCalls()) != 3 {
		t.Errorf("claude ran %d times, want 3", len(r.claudeCalls()))
	}
}

func TestWhenEveryChunkFailsTheErrorIsNotCalledAPartialReview(t *testing.T) {
	r := crossRig(t, sumReplies, envelope("sx", "x", ""))
	failChunks(r, map[int]string{1: "claude exited 1: boom", 2: "claude exited 1: boom", 3: "claude exited 1: boom"})
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, ok, _ := r.svc.Get("o-r-7-1111111")
	if !ok {
		t.Fatal("review not saved")
	}
	if len(got.Findings) != 0 {
		t.Errorf("findings = %+v, want none", got.Findings)
	}
	if got.Error == "" || !strings.Contains(got.Error, "chunk 1") {
		t.Errorf("error = %q, want a non-empty text naming chunk 1", got.Error)
	}
	if strings.HasPrefix(got.Error, "partial review") {
		t.Errorf("error = %q, nothing was reviewed so it is not partial", got.Error)
	}
	if len(r.claudeCalls()) != 3 {
		t.Errorf("claude ran %d times, want 3", len(r.claudeCalls()))
	}
}

func TestACrossPassFailureKeepsTheChunkFindingsAndSaysSo(t *testing.T) {
	r := crossRig(t, sumReplies, envelope("sx", "x", ""))
	r.fx.fail = func(c call) error {
		if c.Name == "claude" && isCrossCall(c) {
			return errors.New("claude exited 1: cross boom")
		}
		return nil
	}
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, ok, _ := r.svc.Get("o-r-7-1111111")
	if !ok {
		t.Fatal("review not saved")
	}
	want := "partial review: cross-file pass failed: claude exited 1: cross boom"
	if got.Error != want {
		t.Errorf("error = %q, want %q", got.Error, want)
	}
	if len(got.Findings) != 3 {
		t.Errorf("findings = %+v, want the three chunk findings", got.Findings)
	}
	if got.Skipped != "" {
		t.Errorf("skipped = %q, want empty", got.Skipped)
	}
}

func TestAPartialReviewIsNotRetriedByALaterTick(t *testing.T) {
	r := crossRig(t, sumReplies, envelope("sx", "x", ""))
	failChunks(r, map[int]string{2: "claude exited 1: boom"})
	_ = r.svc.Tick(context.Background())
	before := len(r.claudeCalls())
	_ = r.svc.Tick(context.Background())
	if after := len(r.claudeCalls()); after != before {
		t.Fatalf("second tick ran claude %d more times, want 0", after-before)
	}
}

func TestACancelledContextMidReviewSavesNothing(t *testing.T) {
	r := crossRig(t, sumReplies, envelope("sx", "x", ""))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.fx.hook = func(c call) {
		if c.Name == "claude" && chunkNo(c) == 2 {
			cancel()
		}
	}
	_ = r.svc.Tick(ctx)
	if got, ok, _ := r.svc.Get("o-r-7-1111111"); ok {
		t.Fatalf("a cancelled review was stored: %+v", got)
	}
}
