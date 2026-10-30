package reviews

import (
	"context"
	"testing"
)

// rigPR7 is pull request #7 of the rig as the runner sees it after search and view.
func rigPR7() (PR, PRInfo) {
	pr := PR{Repo: "o/r", Number: 7, Title: "Fix it", Author: "ann", URL: "https://x/7"}
	info := PRInfo{SHA: "1111111aaaa", Lines: 50, Files: 2, Title: "Fix it", URL: "https://x/7", Author: "ann"}
	return pr, info
}

func TestOneChunkDiffGetsExactlyOneRunWithTheUsualPrompt(t *testing.T) {
	r := newRig(t)
	onlySmallPR(r)
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	calls := r.claudeCalls()
	if len(calls) != 1 {
		t.Fatalf("claude ran %d times, want 1", len(calls))
	}
	pr, info := rigPR7()
	want := buildPrompt(pr, info, "diff --git a/a.go b/a.go\n+x")
	if calls[0].Stdin != want {
		t.Errorf("stdin = %q\nwant   %q", calls[0].Stdin, want)
	}
	got, ok, _ := r.svc.Get("o-r-7-1111111")
	if !ok || got.Summary != "two issues" || len(got.Findings) != 2 || got.Transcript != "sess-9" || got.Account != "home" {
		t.Fatalf("review = %+v", got)
	}
}
