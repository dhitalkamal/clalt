package reviews

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// answerHugeState makes gh say what the state lookup of #8 returns. it goes
// in front of the rig rules, whose "pr view 8 " rule would also match it.
func answerHugeState(r *rig, out string, err error) {
	r.fx.rules = append([]fakeRule{{has: "pr view 8 --repo o/r --json state", out: out, err: err}}, r.fx.rules...)
}

// wantRefreshedOnly checks claude did not run and the record is the same as
// before except for the state, which Tick refreshed.
func wantRefreshedOnly(t *testing.T, r *rig, before Review, state string) {
	t.Helper()
	if n := len(r.claudeCalls()); n != 0 {
		t.Fatalf("claude ran %d times, want 0", n)
	}
	got, ok, err := r.svc.Get(hugeID)
	if err != nil || !ok {
		t.Fatalf("get: %v %v", ok, err)
	}
	if got.State != state {
		t.Errorf("state = %q, want %q", got.State, state)
	}
	if got.StateCheckedAt == nil {
		t.Errorf("stateCheckedAt is nil, want the refresh time")
	}
	want := before
	want.State, want.StateCheckedAt, got.StateCheckedAt = state, nil, nil
	if !reflect.DeepEqual(got, want) {
		t.Errorf("record changed beyond state:\ngot  %+v\nwant %+v", got, want)
	}
}

func TestTickDoesNotRetryAStoredSizeSkipOfAPRThatMergedOrClosedSince(t *testing.T) {
	for _, c := range []struct{ name, gh, state string }{
		{"merged", `{"state":"MERGED"}`, StateMerged},
		{"closed", `{"state":"CLOSED"}`, StateClosed},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			onlyHugePR(r)
			answerHugeState(r, c.gh, nil)
			seedHuge(t, r, oldSizeSkip(StateOpen))
			stored, _, _ := r.svc.Get(hugeID)
			if err := r.svc.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			wantRefreshedOnly(t, r, stored, c.state)
		})
	}
}

func TestTickDoesNotRetryAFailedReviewOfAPRThatMergedOrClosedSince(t *testing.T) {
	for _, c := range []struct{ name, gh, state string }{
		{"merged", `{"state":"MERGED"}`, StateMerged},
		{"closed", `{"state":"CLOSED"}`, StateClosed},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			onlyHugePR(r)
			answerHugeState(r, c.gh, nil)
			seedHuge(t, r, failedRecord("claude exited 1: limit reached", StateOpen))
			stored, _, _ := r.svc.Get(hugeID)
			if err := r.svc.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			wantRefreshedOnly(t, r, stored, c.state)
		})
	}
}

func TestTickRetriesAFailedReviewWhenTheFreshStateIsStillOpen(t *testing.T) {
	r := newRig(t)
	onlyHugePR(r)
	answerHugeState(r, `{"state":"OPEN"}`, nil)
	seedHuge(t, r, failedRecord("claude exited 1: limit reached", StateOpen))
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	wantFailedRetried(t, r)
}

func TestTickFallsBackOnTheStoredStateWhenTheStateLookupFails(t *testing.T) {
	r := newRig(t)
	onlyHugePR(r)
	answerHugeState(r, "", errors.New("gh: boom"))
	seedHuge(t, r, failedRecord("claude exited 1: limit reached", StateOpen))
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	wantFailedRetried(t, r)
}
