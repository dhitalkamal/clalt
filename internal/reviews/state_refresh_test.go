package reviews

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

// stateRig is a service over temp folders with a fake gh that only answers
// pr view state lookups. reviews may be disabled, refresh must not care.
type stateRig struct {
	t     *testing.T
	fx    *fakeExec
	svc   *Service
	logs  int
	clock time.Time
}

func newStateRig(t *testing.T, enabled bool) *stateRig {
	t.Helper()
	r := &stateRig{t: t, fx: &fakeExec{}, clock: time.Unix(9000, 0).UTC()}
	r.svc = &Service{
		Dir: t.TempDir(), GH: GH{X: r.fx},
		Now:  func() time.Time { return r.clock },
		Logf: func(string, ...any) { r.logs++ },
	}
	if err := r.svc.SaveSettings(Config{Enabled: enabled}); err != nil {
		t.Fatal(err)
	}
	return r
}

// answer makes gh say out (or fail with err) for the state lookup of repo#number.
func (r *stateRig) answer(repo string, number int, out string, err error) {
	r.t.Helper()
	r.fx.on("pr view "+strconv.Itoa(number)+" --repo "+repo+" --json state", out, err)
}

// put stores a review of repo#number at sha with a state and check time.
func (r *stateRig) put(repo string, number int, sha, state string, checked *time.Time) string {
	r.t.Helper()
	rv := Review{
		Repo: repo, Number: number, SHA: sha, Title: "t" + sha, Summary: "s" + sha,
		Author: "ann", URL: "https://x/" + sha, Findings: []Finding{{Severity: "low", Title: "f"}},
		State: state, StateCheckedAt: checked,
	}
	if err := r.svc.store().Save(rv); err != nil {
		r.t.Fatal(err)
	}
	return rv.ID()
}

func (r *stateRig) get(id string) Review {
	r.t.Helper()
	rv, ok, err := r.svc.Get(id)
	if err != nil || !ok {
		r.t.Fatalf("get %s: ok=%v err=%v", id, ok, err)
	}
	return rv
}

func (r *stateRig) wantState(id, state string, checked *time.Time) {
	r.t.Helper()
	rv := r.get(id)
	if rv.State != state {
		r.t.Errorf("%s state = %q, want %q", id, rv.State, state)
	}
	switch {
	case checked == nil && rv.StateCheckedAt != nil:
		r.t.Errorf("%s checkedAt = %v, want nil", id, rv.StateCheckedAt)
	case checked != nil && (rv.StateCheckedAt == nil || !rv.StateCheckedAt.Equal(*checked)):
		r.t.Errorf("%s checkedAt = %v, want %v", id, rv.StateCheckedAt, checked)
	}
}

func (r *stateRig) stateCalls() []string {
	var out []string
	for _, l := range r.fx.lines() {
		if strings.Contains(l, "pr view") {
			out = append(out, l)
		}
	}
	return out
}

func TestRefreshStatesAsksGhOnlyForUnknownAndOpenPRs(t *testing.T) {
	r := newStateRig(t, true)
	old := time.Unix(100, 0).UTC()
	r.put("o/r", 1, "1111111", StateUnknown, nil)
	r.put("o/r", 2, "2222222", StateOpen, &old)
	r.put("o/r", 3, "3333333", StateMerged, &old)
	r.put("o/r", 4, "4444444", StateClosed, &old)
	// pr 5 has one merged and one unknown commit: the merged one settles it
	r.put("o/r", 5, "5555555", StateMerged, &old)
	r.put("o/r", 5, "5555556", StateUnknown, nil)
	r.answer("o/r", 1, `{"state":"OPEN"}`, nil)
	r.answer("o/r", 2, `{"state":"OPEN"}`, nil)

	r.svc.RefreshStates(context.Background())

	want := []string{
		"gh pr view 1 --repo o/r --json state",
		"gh pr view 2 --repo o/r --json state",
	}
	got := r.stateCalls()
	if len(got) != len(want) {
		t.Fatalf("gh calls = %v, want %v", got, want)
	}
	seen := map[string]bool{}
	for _, l := range got {
		seen[l] = true
	}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("missing call %q in %v", w, got)
		}
	}
	// the settled ones are untouched
	r.wantState("o-r-3-3333333", StateMerged, &old)
	r.wantState("o-r-4-4444444", StateClosed, &old)
	r.wantState("o-r-5-5555556", StateUnknown, nil)
}

func TestRefreshStatesUpdatesEveryReviewOfOnePR(t *testing.T) {
	r := newStateRig(t, true)
	old := time.Unix(100, 0).UTC()
	a := r.put("o/r", 7, "aaaaaaa", StateOpen, &old)
	b := r.put("o/r", 7, "bbbbbbb", StateUnknown, nil)
	r.answer("o/r", 7, `{"state":"MERGED"}`, nil)

	r.svc.RefreshStates(context.Background())

	if n := len(r.stateCalls()); n != 1 {
		t.Fatalf("gh asked %d times for one pr, want 1: %v", n, r.stateCalls())
	}
	now := r.clock
	r.wantState(a, StateMerged, &now)
	r.wantState(b, StateMerged, &now)
	// nothing else in the review changed
	got := r.get(a)
	if got.Title != "taaaaaaa" || got.Summary != "saaaaaaa" || got.Author != "ann" || len(got.Findings) != 1 || got.SHA != "aaaaaaa" {
		t.Errorf("review changed beyond state: %+v", got)
	}
}

func TestRefreshStatesKeepsOpenButRefreshesCheckTime(t *testing.T) {
	r := newStateRig(t, true)
	old := time.Unix(100, 0).UTC()
	id := r.put("o/r", 2, "2222222", StateOpen, &old)
	r.answer("o/r", 2, `{"state":"OPEN"}`, nil)

	r.svc.RefreshStates(context.Background())

	now := r.clock
	r.wantState(id, StateOpen, &now)
}

func TestRefreshStatesTurnsAnUnknownOldReviewIntoClosed(t *testing.T) {
	r := newStateRig(t, true)
	// an old-format file has no state and no check time
	id := r.put("o/r", 3, "3333333", "", nil)
	r.answer("o/r", 3, `{"state":"CLOSED"}`, nil)

	r.svc.RefreshStates(context.Background())

	now := r.clock
	r.wantState(id, StateClosed, &now)
}

func TestRefreshStatesLeavesReviewsAloneWhenGhFails(t *testing.T) {
	r := newStateRig(t, true)
	old := time.Unix(100, 0).UTC()
	a := r.put("o/r", 1, "1111111", StateOpen, &old)
	b := r.put("o/r", 1, "1111112", StateUnknown, nil)
	r.answer("o/r", 1, "", errors.New("gh: network down"))

	r.svc.RefreshStates(context.Background())

	r.wantState(a, StateOpen, &old)
	r.wantState(b, StateUnknown, nil)
	if r.logs != 0 {
		t.Errorf("Logf called %d times on a failed lookup, want 0", r.logs)
	}
}

func TestRefreshStatesLeavesReviewsAloneOnAnOddAnswer(t *testing.T) {
	r := newStateRig(t, true)
	old := time.Unix(100, 0).UTC()
	a := r.put("o/r", 1, "1111111", StateOpen, &old)
	b := r.put("o/r", 2, "2222222", StateUnknown, nil)
	r.answer("o/r", 1, `{"state":"DRAFTY"}`, nil)
	r.answer("o/r", 2, `not json at all`, nil)

	r.svc.RefreshStates(context.Background())

	r.wantState(a, StateOpen, &old)
	r.wantState(b, StateUnknown, nil)
	if r.logs != 0 {
		t.Errorf("Logf called %d times on an odd answer, want 0", r.logs)
	}
}

func TestRefreshStatesStillRefreshesWhenReviewsAreDisabled(t *testing.T) {
	r := newStateRig(t, false)
	id := r.put("o/r", 1, "1111111", StateUnknown, nil)
	r.answer("o/r", 1, `{"state":"MERGED"}`, nil)

	r.svc.RefreshStates(context.Background())

	now := r.clock
	r.wantState(id, StateMerged, &now)
}

func TestRunRefreshesStatesAfterTheFirstTick(t *testing.T) {
	r := newStateRig(t, false)
	id := r.put("o/r", 1, "1111111", StateUnknown, nil)
	r.answer("o/r", 1, `{"state":"MERGED"}`, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.svc.Run(ctx)
		close(done)
	}()
	deadline := time.Now().Add(5 * time.Second)
	merged := false
	for time.Now().Before(deadline) {
		if rv, ok, _ := r.svc.Get(id); ok && rv.State == StateMerged {
			merged = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
	if !merged {
		t.Fatal("review never became merged: Run does not refresh states after a tick")
	}
}
