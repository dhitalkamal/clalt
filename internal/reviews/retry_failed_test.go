package reviews

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// failedRecord is a stored run error with no attempts fields, like old files.
func failedRecord(errText, state string) Review {
	rv := Review{Error: errText, Summary: "old", Author: "bo", URL: "https://x/8", State: state}
	if state != "" {
		at := time.Unix(100, 0).UTC()
		rv.StateCheckedAt = &at
	}
	return rv
}

// wantFailedRetried checks one claude run replaced the failure in the same file.
func wantFailedRetried(t *testing.T, r *rig) {
	t.Helper()
	if n := len(r.claudeCalls()); n != 1 {
		t.Fatalf("claude ran %d times, want 1 (the failed review is retried)", n)
	}
	got, ok, err := r.svc.Get(hugeID)
	if err != nil || !ok {
		t.Fatalf("get: %v %v", ok, err)
	}
	if got.Error != "" || len(got.Findings) != 2 || got.Summary != "two issues" {
		t.Fatalf("record after retry = %+v, want a normal review with findings and no error", got)
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

// wantUntouched checks claude did not run and the file is byte for byte the same.
func wantUntouched(t *testing.T, r *rig, path string, before []byte) {
	t.Helper()
	if n := len(r.claudeCalls()); n != 0 {
		t.Fatalf("claude ran %d times, want 0", n)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("record changed: err=%v\ngot  %s\nwant %s", err, after, before)
	}
}

func TestTickRetriesAFailedReviewWhoseStateIsOpen(t *testing.T) {
	r := newRig(t)
	onlyHugePR(r)
	seedHuge(t, r, failedRecord("claude exited 1: limit reached", StateOpen))
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	wantFailedRetried(t, r)
}

func TestTickRetriesAFailedReviewWhoseStateIsUnknownOrEmpty(t *testing.T) {
	for name, state := range map[string]string{"unknown": StateUnknown, "empty": ""} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t)
			onlyHugePR(r)
			seedHuge(t, r, failedRecord("claude exited 1: limit reached", state))
			if err := r.svc.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			wantFailedRetried(t, r)
		})
	}
}

func TestTickDoesNotRetryAPartialReview(t *testing.T) {
	at := time.Unix(100, 0).UTC()
	rv := Review{
		Error:          partialPrefix + "chunk 2 failed",
		Findings:       []Finding{{Severity: "low", Title: "kept"}},
		Summary:        "partial",
		State:          StateOpen,
		StateCheckedAt: &at,
	}
	r := newRig(t)
	onlyHugePR(r)
	path := seedHuge(t, r, rv)
	before, _ := os.ReadFile(path)
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	wantUntouched(t, r, path, before)
}

func TestTickRetriesAnAllChunksFailedRecord(t *testing.T) {
	r := newRig(t)
	onlyHugePR(r)
	seedHuge(t, r, failedRecord("all 3 chunks failed: claude exited 1", StateOpen))
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	wantFailedRetried(t, r)
}

func TestTickDoesNotRetryAFailedReviewWhoseStateIsMergedOrClosed(t *testing.T) {
	for _, state := range []string{StateMerged, StateClosed} {
		t.Run(state, func(t *testing.T) {
			r := newRig(t)
			onlyHugePR(r)
			path := seedHuge(t, r, failedRecord("claude exited 1: limit reached", state))
			before, _ := os.ReadFile(path)
			if err := r.svc.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			wantUntouched(t, r, path, before)
		})
	}
}

func TestTickDoesNotRetryASuccessfulReview(t *testing.T) {
	at := time.Unix(100, 0).UTC()
	rv := Review{Summary: "fine", Findings: []Finding{{Severity: "low", Title: "f"}}, State: StateOpen, StateCheckedAt: &at}
	r := newRig(t)
	onlyHugePR(r)
	path := seedHuge(t, r, rv)
	before, _ := os.ReadFile(path)
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	wantUntouched(t, r, path, before)
}
