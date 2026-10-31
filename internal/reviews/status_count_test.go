package reviews

import "testing"

// the badge count is the reviews still in the open tab: merged and closed pull requests are history
func TestStatusCountsOnlyReviewsThatAreNotMergedOrClosed(t *testing.T) {
	r := newRig(t)
	for i, st := range []string{StateOpen, StateMerged, StateClosed, "", StateUnknown, StateOpen} {
		rv := Review{Repo: "o/r", Number: 10 + i, SHA: "1111111aaaa", Title: "t", Summary: "s", State: st}
		if err := r.svc.store().Save(rv); err != nil {
			t.Fatal(err)
		}
	}
	if got := r.svc.Status().Count; got != 4 {
		t.Fatalf("count = %d, want 4 (two open, one unknown, one stored before state existed)", got)
	}
}
