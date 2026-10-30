package reviews

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

const giveUpNote = "gave up after 5 attempts"

// getHuge loads the stored record for the huge pr.
func getHuge(t *testing.T, r *rig) Review {
	t.Helper()
	got, ok, err := r.svc.Get(hugeID)
	if err != nil || !ok {
		t.Fatalf("get: %v %v", ok, err)
	}
	return got
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestFifthFailureIsStoredWithAttemptFiveNoNextRetryAndTheGiveUpNote(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, true)
	seedAttempt(t, r, 4, now)
	tickOnce(t, r)
	got := getHuge(t, r)
	if got.Attempts != 5 {
		t.Fatalf("attempts = %d, want 5", got.Attempts)
	}
	if got.NextRetryAt != nil {
		t.Fatalf("nextRetryAt = %v, want nil", got.NextRetryAt)
	}
	for _, want := range []string{"claude exited 1: limit reached", giveUpNote} {
		if !strings.Contains(got.Error, want) {
			t.Fatalf("error = %q, want it to contain %q", got.Error, want)
		}
	}
}

func TestGiveUpNoteSurvivesTrimmingOfALongCause(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, false)
	r.fx.fail = func(c call) error {
		if c.Name == "claude" {
			return errors.New(strings.Repeat("filler ", 300) + "distinct-tail-xyz")
		}
		return nil
	}
	seedAttempt(t, r, 4, now)
	tickOnce(t, r)
	got := getHuge(t, r)
	if got.Attempts != 5 {
		t.Fatalf("attempts = %d, want 5", got.Attempts)
	}
	if !strings.Contains(got.Error, giveUpNote) {
		t.Fatalf("error = %q, want it to contain %q", got.Error, giveUpNote)
	}
}

func TestRecordAtAttemptFiveIsNeverRetriedEvenMuchLater(t *testing.T) {
	for name, next := range map[string]*time.Time{
		"past": ptrTime(time.Unix(1000, 0).UTC()),
		"nil":  nil,
	} {
		t.Run(name, func(t *testing.T) {
			now := time.Unix(5000, 0).UTC()
			r := backoffRig(t, &now, false)
			rv := failedRecord("claude exited 1: limit reached", StateOpen)
			rv.Attempts = 5
			rv.NextRetryAt = next
			path := seedHuge(t, r, rv)
			before, _ := os.ReadFile(path)
			for i := 0; i < 4; i++ {
				tickOnce(t, r)
			}
			now = now.Add(30 * 24 * time.Hour)
			for i := 0; i < 4; i++ {
				tickOnce(t, r)
			}
			wantUntouched(t, r, path, before)
		})
	}
}

func TestFailuresOneToFourDoNotCarryTheGiveUpNote(t *testing.T) {
	for attempts := 0; attempts <= 3; attempts++ {
		now := time.Unix(5000, 0).UTC()
		r := backoffRig(t, &now, true)
		if attempts > 0 {
			seedAttempt(t, r, attempts, now)
		}
		tickOnce(t, r)
		got := getHuge(t, r)
		if got.Attempts != attempts+1 {
			t.Fatalf("attempts = %d, want %d", got.Attempts, attempts+1)
		}
		if strings.Contains(got.Error, "gave up") {
			t.Fatalf("attempt %d error = %q, want no give-up note", got.Attempts, got.Error)
		}
	}
}

func TestRetrySucceedingAtAttemptFourStoresANormalReview(t *testing.T) {
	now := time.Unix(5000, 0).UTC()
	r := backoffRig(t, &now, false)
	seedAttempt(t, r, 4, now)
	tickOnce(t, r)
	wantFailedRetried(t, r)
	got := getHuge(t, r)
	if got.Attempts != 0 || got.NextRetryAt != nil || strings.Contains(got.Error, "gave up") {
		t.Fatalf("record = %+v, want attempts 0, no next retry, no note", got)
	}
}
