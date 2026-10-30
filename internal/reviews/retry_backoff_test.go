package reviews

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// backoffRig is a rig whose clock the test can move, with claude failing on demand.
func backoffRig(t *testing.T, now *time.Time, claudeFails bool) *rig {
	t.Helper()
	r := newRig(t)
	onlyHugePR(r)
	r.svc.Now = func() time.Time { return *now }
	if claudeFails {
		r.fx.fail = func(c call) error {
			if c.Name == "claude" {
				return errors.New("claude exited 1: limit reached")
			}
			return nil
		}
	}
	return r
}

// seedAttempt stores a failed record with attempts and a due time.
func seedAttempt(t *testing.T, r *rig, attempts int, next time.Time) string {
	t.Helper()
	rv := failedRecord("claude exited 1: limit reached", StateOpen)
	rv.Attempts = attempts
	rv.NextRetryAt = &next
	return seedHuge(t, r, rv)
}

// wantBackoff checks the stored record is failed with attempts and next retry time.
func wantBackoff(t *testing.T, r *rig, attempts int, next time.Time) {
	t.Helper()
	got, ok, err := r.svc.Get(hugeID)
	if err != nil || !ok {
		t.Fatalf("get: %v %v", ok, err)
	}
	if got.Error == "" {
		t.Fatalf("record = %+v, want a failed review", got)
	}
	if got.Attempts != attempts {
		t.Fatalf("attempts = %d, want %d", got.Attempts, attempts)
	}
	if got.NextRetryAt == nil || !got.NextRetryAt.Equal(next) {
		t.Fatalf("nextRetryAt = %v, want %v", got.NextRetryAt, next)
	}
}

func tickOnce(t *testing.T, r *rig) {
	t.Helper()
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestFirstFailureStoresAttemptOneAndRetryInFiveMinutes(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, true)
	tickOnce(t, r)
	wantBackoff(t, r, 1, now.Add(5*time.Minute))
}

func TestFailedRetryAtAttemptOneStoresAttemptTwoAndWaitsFifteenMinutes(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, true)
	seedAttempt(t, r, 1, now)
	tickOnce(t, r)
	wantBackoff(t, r, 2, now.Add(15*time.Minute))
}

func TestFailedRetryAtAttemptTwoStoresAttemptThreeAndWaitsOneHour(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, true)
	seedAttempt(t, r, 2, now)
	tickOnce(t, r)
	wantBackoff(t, r, 3, now.Add(time.Hour))
}

func TestFailedRetryAtAttemptThreeStoresAttemptFourAndWaitsThreeHours(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, true)
	seedAttempt(t, r, 3, now)
	tickOnce(t, r)
	wantBackoff(t, r, 4, now.Add(3*time.Hour))
}

func TestFailedRetryAtAttemptFourIsStoredAsFailedWithAttemptFive(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, true)
	seedAttempt(t, r, 4, now)
	tickOnce(t, r)
	got, ok, err := r.svc.Get(hugeID)
	if err != nil || !ok || got.Error == "" || got.Attempts != 5 {
		t.Fatalf("record = %+v ok=%v err=%v, want failed with attempts 5", got, ok, err)
	}
}

func TestFailedReviewIsNotRetriedOneSecondBeforeItsNextRetryTime(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, false)
	path := seedAttempt(t, r, 1, now.Add(time.Second))
	before, _ := os.ReadFile(path)
	tickOnce(t, r)
	wantUntouched(t, r, path, before)
}

func TestFailedReviewIsRetriedAtExactlyItsNextRetryTime(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, false)
	seedAttempt(t, r, 1, now)
	tickOnce(t, r)
	wantFailedRetried(t, r)
}

func TestFailedReviewIsRetriedOnceItsNextRetryTimeHasPassed(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, false)
	seedAttempt(t, r, 2, now.Add(-time.Hour))
	tickOnce(t, r)
	wantFailedRetried(t, r)
}

func TestFailedReviewBecomesDueWhenTheClockReachesItsNextRetryTime(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, false)
	path := seedAttempt(t, r, 1, now.Add(time.Minute))
	before, _ := os.ReadFile(path)
	tickOnce(t, r)
	wantUntouched(t, r, path, before)
	now = now.Add(time.Minute)
	tickOnce(t, r)
	wantFailedRetried(t, r)
}

func TestLegacyFailedRecordIsRetriedAtOnceAndAGoodRetryHasNoAttemptFields(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, false)
	seedHuge(t, r, failedRecord("claude exited 1: limit reached", StateOpen))
	tickOnce(t, r)
	wantFailedRetried(t, r)
	got, _, _ := r.svc.Get(hugeID)
	if got.Attempts != 0 || got.NextRetryAt != nil {
		t.Fatalf("attempts = %d next = %v, want 0 and nil after a good retry", got.Attempts, got.NextRetryAt)
	}
}

func TestLegacyFailedRecordFailingAgainIsSavedAsAttemptTwoWithFifteenMinutes(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, true)
	seedHuge(t, r, failedRecord("claude exited 1: limit reached", StateOpen))
	tickOnce(t, r)
	if n := len(r.claudeCalls()); n != 1 {
		t.Fatalf("claude ran %d times, want 1", n)
	}
	wantBackoff(t, r, 2, now.Add(15*time.Minute))
}

func TestSuccessfulReviewFileHasNoAttemptsOrNextRetryKeys(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, false)
	tickOnce(t, r)
	raw, err := os.ReadFile(filepath.Join(r.dir, hugeID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"attempts"`, `"nextRetryAt"`} {
		if strings.Contains(string(raw), key) {
			t.Fatalf("file has %s, want it omitted:\n%s", key, raw)
		}
	}
}

func TestFailedReviewFilePersistsAttemptsAndNextRetryAt(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, true)
	tickOnce(t, r)
	raw, err := os.ReadFile(filepath.Join(r.dir, hugeID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"attempts"`, `"nextRetryAt"`} {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("file lacks %s:\n%s", key, raw)
		}
	}
}
