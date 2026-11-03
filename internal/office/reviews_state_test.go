package office

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/reviews"
)

// stateReview is oneReview with the given state fields set.
func stateReview(state string, checked *time.Time) reviews.Review {
	r := oneReview()
	r.State = state
	r.StateCheckedAt = checked
	return r
}

// listRow gets /api/reviews and returns the only review row as a raw map.
func listRow(t *testing.T, r reviews.Review) map[string]json.RawMessage {
	t.Helper()
	f := &fakeReviews{list: []reviews.Review{r}, status: reviews.Status{Enabled: true, Count: 1}}
	rec := getAuthed(reviewEnv(t, f), "/api/reviews")
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var got struct {
		Reviews []map[string]json.RawMessage `json:"reviews"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Reviews) != 1 {
		t.Fatalf("want one row: %s", rec.Body)
	}
	return got.Reviews[0]
}

// singleRow gets /api/reviews/<id> and returns the body as a raw map.
func singleRow(t *testing.T, r reviews.Review) map[string]json.RawMessage {
	t.Helper()
	f := &fakeReviews{list: []reviews.Review{r}}
	rec := getAuthed(reviewEnv(t, f), "/api/reviews/"+r.ID())
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

// wantState asserts the state value and the stateCheckedAt raw json.
func wantState(t *testing.T, row map[string]json.RawMessage, state, checked string) {
	t.Helper()
	s, ok := row["state"]
	if !ok {
		t.Fatalf("state key missing: %v", row)
	}
	if string(s) != `"`+state+`"` {
		t.Errorf("state = %s, want %q", s, state)
	}
	c, ok := row["stateCheckedAt"]
	if !ok {
		t.Fatalf("stateCheckedAt key missing (must be present, null when never checked): %v", row)
	}
	if string(c) != checked {
		t.Errorf("stateCheckedAt = %s, want %s", c, checked)
	}
}

func TestListRowOfMergedReviewShowsStateAndCheckTime(t *testing.T) {
	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	wantState(t, listRow(t, stateReview(reviews.StateMerged, &at)), "merged", `"2026-03-04T05:06:07Z"`)
}

func TestListRowWithEmptyStateIsUnknownAndCheckTimeNull(t *testing.T) {
	wantState(t, listRow(t, stateReview("", nil)), "unknown", "null")
}

func TestSingleGetWithEmptyStateIsUnknownAndCheckTimeNull(t *testing.T) {
	wantState(t, singleRow(t, stateReview("", nil)), "unknown", "null")
}

func TestSingleGetOfOpenReviewShowsStateAndCheckTime(t *testing.T) {
	at := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	wantState(t, singleRow(t, stateReview(reviews.StateOpen, &at)), "open", `"2026-05-06T07:08:09Z"`)
}

func TestSingleGetOfClosedReviewShowsStateAndCheckTime(t *testing.T) {
	at := time.Date(2026, 6, 7, 8, 9, 10, 0, time.UTC)
	wantState(t, singleRow(t, stateReview(reviews.StateClosed, &at)), "closed", `"2026-06-07T08:09:10Z"`)
}

func TestUnrecognizedStateComesOutUnknownInListAndSingle(t *testing.T) {
	wantState(t, listRow(t, stateReview("weird", nil)), "unknown", "null")
	wantState(t, singleRow(t, stateReview("weird", nil)), "unknown", "null")
}
