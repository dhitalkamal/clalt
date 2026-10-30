package office

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/reviews"
)

// failedReview is oneReview as a failed run with the given retry fields.
func failedReview(attempts int, next *time.Time) reviews.Review {
	r := oneReview()
	r.Error = "boom"
	r.Attempts = attempts
	r.NextRetryAt = next
	return r
}

// wantRetry asserts attempts and nextRetryAt; an empty want means the key must be absent.
func wantRetry(t *testing.T, row map[string]json.RawMessage, attempts, next string) {
	t.Helper()
	for key, want := range map[string]string{"attempts": attempts, "nextRetryAt": next} {
		got, ok := row[key]
		if want == "" {
			if ok {
				t.Errorf("%s = %s, want key absent", key, got)
			}
			continue
		}
		if !ok {
			t.Errorf("%s key missing, want %s: %v", key, want, row)
		} else if string(got) != want {
			t.Errorf("%s = %s, want %s", key, got, want)
		}
	}
}

// wantKeepsExistingKeys asserts the pre-existing row keys are still there.
func wantKeepsExistingKeys(t *testing.T, row map[string]json.RawMessage) {
	t.Helper()
	for _, k := range []string{"id", "repo", "number", "title", "state", "stateCheckedAt", "error", "counts", "handle"} {
		if _, ok := row[k]; !ok {
			t.Errorf("existing key %q missing: %v", k, row)
		}
	}
}

func TestListRowOfFailedReviewShowsAttemptsAndNextRetryTime(t *testing.T) {
	at := time.Date(2026, 7, 8, 9, 10, 11, 0, time.UTC)
	row := listRow(t, failedReview(2, &at))
	wantRetry(t, row, "2", `"2026-07-08T09:10:11Z"`)
	wantKeepsExistingKeys(t, row)
}

func TestSingleGetOfFailedReviewShowsAttemptsAndNextRetryTime(t *testing.T) {
	at := time.Date(2026, 8, 9, 10, 11, 12, 0, time.UTC)
	row := singleRow(t, failedReview(2, &at))
	wantRetry(t, row, "2", `"2026-08-09T10:11:12Z"`)
	wantKeepsExistingKeys(t, row)
}

func TestRowsWithZeroAttemptsAndNoRetryTimeOmitBothKeys(t *testing.T) {
	wantRetry(t, listRow(t, failedReview(0, nil)), "", "")
	wantRetry(t, singleRow(t, failedReview(0, nil)), "", "")
}

func TestGivenUpReviewShowsAttemptsAndNoNextRetryKey(t *testing.T) {
	wantRetry(t, listRow(t, failedReview(5, nil)), "5", "")
	wantRetry(t, singleRow(t, failedReview(5, nil)), "5", "")
}
