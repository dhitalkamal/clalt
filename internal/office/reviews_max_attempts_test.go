package office

import (
	"encoding/json"
	"testing"

	"github.com/dhitalkamal/clalt/internal/reviews"
)

// listBody gets /api/reviews and returns the top-level body as a raw map.
func listBody(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	f := &fakeReviews{list: []reviews.Review{oneReview()}, status: reviews.Status{Enabled: true, Count: 1}}
	rec := getAuthed(reviewEnv(t, f), "/api/reviews")
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestListReplyCarriesMaxAttemptsAsANumberFromTheReviewsPackage(t *testing.T) {
	raw, ok := listBody(t)["maxAttempts"]
	if !ok {
		t.Fatal("maxAttempts key missing from list reply")
	}
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		t.Fatalf("maxAttempts = %s, want a number: %v", raw, err)
	}
	if want := reviews.MaxAttempts(); n != want {
		t.Fatalf("maxAttempts = %d, want %d", n, want)
	}
}

func TestListReplyKeepsItsExistingTopLevelKeys(t *testing.T) {
	body := listBody(t)
	for _, k := range []string{"enabled", "running", "lastChecked", "reviews"} {
		if _, ok := body[k]; !ok {
			t.Errorf("key %q missing from list reply: %v", k, body)
		}
	}
}
