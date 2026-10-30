package reviews

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const hugeID = "o-r-8-2222222"

// sizeSkip is the text older versions stored for a big pull request.
const sizeSkip = "not reviewed: over 1500 changed lines"

// onlyHugePR narrows the rig's search to #8 so a test counts calls for one review.
func onlyHugePR(r *rig) {
	r.fx.rules = append([]fakeRule{{has: "search prs", out: `[{"number":8,"title":"Huge","url":"https://x/8","isDraft":false,"author":{"login":"bo"},"repository":{"nameWithOwner":"o/r"}}]`}}, r.fx.rules...)
}

// seedHuge stores an old record for #8 at the rig's sha and returns its file path.
func seedHuge(t *testing.T, r *rig, rv Review) string {
	t.Helper()
	rv.Repo, rv.Number, rv.SHA = "o/r", 8, "2222222bbbb"
	if rv.Title == "" {
		rv.Title = "Huge"
	}
	if err := r.svc.store().Save(rv); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(r.dir, hugeID+".json")
}

func oldSizeSkip(state string) Review {
	at := time.Unix(100, 0).UTC()
	rv := Review{Skipped: sizeSkip, State: state, Summary: "old", Author: "bo", URL: "https://x/8"}
	if state != "" {
		rv.StateCheckedAt = &at
	}
	return rv
}

func wantRetried(t *testing.T, r *rig) {
	t.Helper()
	if n := len(r.claudeCalls()); n != 1 {
		t.Fatalf("claude ran %d times, want 1 (the size skip is retried)", n)
	}
	got, ok, err := r.svc.Get(hugeID)
	if err != nil || !ok {
		t.Fatalf("get: %v %v", ok, err)
	}
	if got.Skipped != "" || len(got.Findings) != 2 || got.Summary != "two issues" || got.State != StateOpen {
		t.Fatalf("record after retry = %+v, want a normal open review", got)
	}
	var n int
	for _, f := range reviewFiles(t, r.dir) {
		if strings.HasPrefix(f, "o-r-8-") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("files for #8 = %d, want 1", n)
	}
}

func TestTickRetriesAStoredSizeSkipWhoseStateIsOpen(t *testing.T) {
	r := newRig(t)
	onlyHugePR(r)
	seedHuge(t, r, oldSizeSkip(StateOpen))
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	wantRetried(t, r)
}

func TestTickRetriesAStoredSizeSkipWithNoStateAtAll(t *testing.T) {
	r := newRig(t)
	onlyHugePR(r)
	path := seedHuge(t, r, oldSizeSkip(""))
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), `"state"`) {
		t.Fatalf("setup: file should have no state: %s", raw)
	}
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	wantRetried(t, r)
}

func TestTickRetriesAStoredSizeSkipWhoseStateIsUnknown(t *testing.T) {
	r := newRig(t)
	onlyHugePR(r)
	seedHuge(t, r, oldSizeSkip(StateUnknown))
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	wantRetried(t, r)
}

func TestTickLeavesAMergedOrClosedSizeSkipAsHistory(t *testing.T) {
	for _, state := range []string{StateMerged, StateClosed} {
		t.Run(state, func(t *testing.T) {
			r := newRig(t)
			onlyHugePR(r)
			path := seedHuge(t, r, oldSizeSkip(state))
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := r.svc.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			if n := len(r.claudeCalls()); n != 0 {
				t.Fatalf("claude ran %d times for a %s pr, want 0", n, state)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(before) {
				t.Fatalf("record changed: err=%v\ngot  %s\nwant %s", err, after, before)
			}
		})
	}
}

func TestTickDoesNotRetryAReviewThatIsNotASizeSkip(t *testing.T) {
	at := time.Unix(100, 0).UTC()
	cases := map[string]Review{
		"normal findings": {Summary: "fine", Findings: []Finding{{Severity: "low", Title: "f"}}, State: StateOpen, StateCheckedAt: &at},
		"run error":       {Error: "claude exited 1: limit reached", State: StateOpen, StateCheckedAt: &at},
		"other skip":      {Skipped: "not reviewed: something else", State: StateOpen, StateCheckedAt: &at},
	}
	for name, rv := range cases {
		t.Run(name, func(t *testing.T) {
			r := newRig(t)
			onlyHugePR(r)
			path := seedHuge(t, r, rv)
			before, _ := os.ReadFile(path)
			if err := r.svc.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			if n := len(r.claudeCalls()); n != 0 {
				t.Fatalf("claude ran %d times, want 0", n)
			}
			after, _ := os.ReadFile(path)
			if string(after) != string(before) {
				t.Fatalf("record changed:\ngot  %s\nwant %s", after, before)
			}
		})
	}
}

func TestRetryingASizeSkipNeverDeletesAFile(t *testing.T) {
	r := newRig(t)
	seedHuge(t, r, oldSizeSkip(StateOpen))
	// an unrelated review of #7 and an older commit of #8 sit beside it
	if err := r.svc.store().Save(Review{Repo: "o/r", Number: 7, SHA: "1111111aaaa", Title: "Fix it", Summary: "s", State: StateOpen}); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.store().Save(Review{Repo: "o/r", Number: 8, SHA: "9999999dddd", Title: "Huge", Summary: "s", State: StateOpen}); err != nil {
		t.Fatal(err)
	}
	before := reviewFiles(t, r.dir)
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := len(r.claudeCalls()); n != 1 {
		t.Fatalf("claude ran %d times, want 1 (only the size skip is retried)", n)
	}
	after := map[string]bool{}
	for _, f := range reviewFiles(t, r.dir) {
		after[f] = true
	}
	for _, f := range before {
		if !after[f] {
			t.Errorf("file %s is gone after the retry", f)
		}
	}
}

func TestSecondTickAfterARetryDoesNotRunClaudeAgain(t *testing.T) {
	r := newRig(t)
	onlyHugePR(r)
	seedHuge(t, r, oldSizeSkip(StateOpen))
	_ = r.svc.Tick(context.Background())
	_ = r.svc.Tick(context.Background())
	if n := len(r.claudeCalls()); n != 1 {
		t.Fatalf("claude ran %d times over two ticks, want 1", n)
	}
}
