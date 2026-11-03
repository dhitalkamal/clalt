package reviews

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

// stateLookups counts the gh state lookups a fake exec saw for repo#number.
func stateLookups(fx *fakeExec, repo string, number int) int {
	want := "pr view " + strconv.Itoa(number) + " --repo " + repo + " --json state"
	n := 0
	for _, l := range fx.lines() {
		if strings.Contains(l, want) {
			n++
		}
	}
	return n
}

func TestPollAsksGhOncePerStoredOpenPRWhenReviewsAreEnabled(t *testing.T) {
	r := newRig(t)
	onlyHugePR(r)
	answerHugeState(r, `{"state":"OPEN"}`, nil)
	seedHuge(t, r, oldSizeSkip(StateOpen))

	if err := r.svc.poll(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := stateLookups(r.fx, "o/r", 8); got != 1 {
		t.Errorf("state lookups = %d, want 1: %v", got, r.fx.lines())
	}
}

func TestPollAsksGhOncePerStoredOpenPRWhenReviewsAreDisabled(t *testing.T) {
	r := newStateRig(t, false)
	r.put("o/r", 1, "1111111", StateOpen, nil)
	r.put("o/r", 2, "2222222", StateOpen, nil)
	r.answer("o/r", 1, `{"state":"OPEN"}`, nil)
	r.answer("o/r", 2, `{"state":"OPEN"}`, nil)

	if err := r.svc.poll(context.Background()); err != nil {
		t.Fatal(err)
	}

	for _, n := range []int{1, 2} {
		if got := stateLookups(r.fx, "o/r", n); got != 1 {
			t.Errorf("state lookups for #%d = %d, want 1: %v", n, got, r.fx.lines())
		}
	}
	if got := len(r.stateCalls()); got != 2 {
		t.Errorf("total state lookups = %d, want 2: %v", got, r.stateCalls())
	}
}

func TestPollUsesTheRefreshBeforeHandlingWhenThePRMergedSince(t *testing.T) {
	r := newRig(t)
	onlyHugePR(r)
	answerHugeState(r, `{"state":"MERGED"}`, nil)
	seedHuge(t, r, oldSizeSkip(StateOpen))
	stored, _, _ := r.svc.Get(hugeID)

	if err := r.svc.poll(context.Background()); err != nil {
		t.Fatal(err)
	}

	wantRefreshedOnly(t, r, stored, StateMerged)
	if got := stateLookups(r.fx, "o/r", 8); got != 1 {
		t.Errorf("state lookups = %d, want 1: %v", got, r.fx.lines())
	}
}
