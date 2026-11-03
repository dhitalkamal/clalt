package reviews

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// goodRecord is a stored normal review of #8: no error, no skip.
func goodRecord() Review {
	return Review{Summary: "fine", Author: "bo", URL: "https://x/8", State: StateOpen}
}

func TestASuccessfulReviewIsNeverRetriedOverManyPollsAndAFarFutureClock(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, false)
	path := seedHuge(t, r, goodRecord())
	before, _ := os.ReadFile(path)
	for i := 0; i < 30; i++ {
		tickOnce(t, r)
		now = now.Add(10 * 365 * 24 * time.Hour)
	}
	wantUntouched(t, r, path, before)
}

func TestAFailedReviewOfAMergedOrClosedPRIsNeverRetried(t *testing.T) {
	for _, state := range []string{StateMerged, StateClosed} {
		t.Run(state, func(t *testing.T) {
			now := time.Unix(5000, 0).UTC()
			r := backoffRig(t, &now, false)
			rv := failedRecord("claude exited 1: limit reached", state)
			rv.Attempts = 1
			past := now.Add(-time.Hour)
			rv.NextRetryAt = &past
			path := seedHuge(t, r, rv)
			before, _ := os.ReadFile(path)
			for i := 0; i < 5; i++ {
				tickOnce(t, r)
				now = now.Add(24 * time.Hour)
			}
			wantUntouched(t, r, path, before)
		})
	}
}

func TestARetryEndingInNoRoomKeepsTheOldFailedRecordByteForByte(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, false)
	r.svc.Accounts = func() []Candidate {
		return []Candidate{{ID: "w", ConfigDir: "/a", FiveHourUsedPct: pct(97)}}
	}
	path := seedAttempt(t, r, 2, now.Add(-time.Minute))
	before, _ := os.ReadFile(path)
	err := r.svc.Tick(context.Background())
	if !errors.Is(err, errNoRoom) {
		t.Fatalf("tick = %v, want errNoRoom", err)
	}
	wantUntouched(t, r, path, before)
}

func TestARetryWithAnAlreadyCancelledContextKeepsTheOldFailedRecord(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, false)
	path := seedAttempt(t, r, 2, now.Add(-time.Minute))
	before, _ := os.ReadFile(path)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = r.svc.Tick(ctx)
	wantUntouched(t, r, path, before)
}

func TestARetryCancelledWhileClaudeRunsKeepsTheOldFailedRecord(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, false)
	path := seedAttempt(t, r, 2, now.Add(-time.Minute))
	before, _ := os.ReadFile(path)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.fx.hook = func(c call) {
		if c.Name == "claude" {
			cancel()
		}
	}
	_ = r.svc.Tick(ctx)
	if n := len(r.claudeCalls()); n != 1 {
		t.Fatalf("claude ran %d times, want 1 (the retry started)", n)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("record changed: err=%v\ngot  %s\nwant %s", err, after, before)
	}
}

func TestRetryCodeNeverDeletesFiles(t *testing.T) {
	fset := token.NewFileSet()
	for _, name := range []string{"retry.go", "backoff.go", "service.go"} {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("guarded file %s must exist: %v", name, err)
		}
		f, err := parser.ParseFile(fset, name, raw, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range deletions(fset, f) {
			t.Errorf("file deletion in retry code: %s", v)
		}
	}
}

func TestTheAttemptCapIsHonoredOverManyPollsWithBigClockJumps(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, true)
	for i := 0; i < 40; i++ {
		tickOnce(t, r)
		now = now.Add(4 * time.Hour)
	}
	if n := len(r.claudeCalls()); n != 5 {
		t.Fatalf("claude ran %d times over 40 polls, want exactly 5", n)
	}
	got := getHuge(t, r)
	if got.Attempts != 5 || got.NextRetryAt != nil || !strings.Contains(got.Error, giveUpNote) {
		t.Fatalf("record = %+v, want attempts 5, no next retry, give-up note", got)
	}
	path := filepath.Join(r.dir, hugeID+".json")
	before, _ := os.ReadFile(path)
	for i := 0; i < 10; i++ {
		now = now.Add(100 * 365 * 24 * time.Hour)
		tickOnce(t, r)
	}
	if n := len(r.claudeCalls()); n != 5 {
		t.Fatalf("claude ran %d times after the cap, want still 5", n)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("record changed after the cap: err=%v", err)
	}
}
