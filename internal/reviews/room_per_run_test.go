package reviews

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const storedID = "o-r-7-1111111"

// roomPlan makes the rig's Accounts func answer by how many times it was
// called so far (1 for the first call). it returns the call counter.
func roomPlan(r *rig, plan func(n int) []Candidate) *int {
	n := new(int)
	r.svc.Accounts = func() []Candidate {
		*n++
		return plan(*n)
	}
	return n
}

// roomOn is one account with plenty of room.
func roomOn(id string) []Candidate {
	return []Candidate{{ID: id, ConfigDir: "/a/" + id, FiveHourUsedPct: pct(5)}}
}

// storedFiles lists the files in the store dir that belong to #7.
func storedFiles(t *testing.T, r *rig) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(r.dir, storedID+"*"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// assertNothingStored checks that the stopped review left no trace for #7.
func assertNothingStored(t *testing.T, r *rig) {
	t.Helper()
	if f := storedFiles(t, r); len(f) != 0 {
		t.Errorf("store has files for #7: %v, want none", f)
	}
	list, _ := r.svc.List()
	for _, rv := range list {
		if rv.Number == 7 {
			t.Errorf("list has #7: %+v, want none (no review, partial or error record)", rv)
		}
	}
	if st := r.svc.Status(); st.Running != nil {
		t.Errorf("running = %+v, want nil", st.Running)
	}
}

func TestAccountsIsPickedOncePerClaudeRunAndEachRunUsesItsOwnPick(t *testing.T) {
	r := crossRig(t, plainReplies, envelope("sx", "x", ""))
	n := roomPlan(r, func(k int) []Candidate {
		// call number k has the most room, the other account is fuller
		id := "pick" + string(rune('0'+k))
		return []Candidate{
			{ID: "old", ConfigDir: "/a/old", FiveHourUsedPct: pct(60)},
			{ID: id, ConfigDir: "/a/" + id, FiveHourUsedPct: pct(5)},
		}
	})
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *n != 4 {
		t.Errorf("Accounts called %d times, want 4 (3 chunks and the cross pass)", *n)
	}
	calls := r.claudeCalls()
	if len(calls) != 4 {
		t.Fatalf("claude ran %d times, want 4", len(calls))
	}
	for i, c := range calls {
		want := "CLAUDE_CONFIG_DIR=/a/pick" + string(rune('1'+i))
		if len(c.Env) != 1 || c.Env[0] != want {
			t.Errorf("run %d env = %v, want [%s]", i+1, c.Env, want)
		}
	}
}

func TestSingleChunkReviewPicksAccountsOnce(t *testing.T) {
	r := newRig(t)
	onlySmallPR(r)
	n := roomPlan(r, func(k int) []Candidate { return roomOn("one") })
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *n != 1 {
		t.Errorf("Accounts called %d times, want 1", *n)
	}
	if got := len(r.claudeCalls()); got != 1 {
		t.Errorf("claude ran %d times, want 1", got)
	}
}

func TestNoRoomBeforeChunkTwoStopsTheReviewAndStoresNothing(t *testing.T) {
	r := crossRig(t, plainReplies, envelope("sx", "x", ""))
	roomPlan(r, func(k int) []Candidate {
		if k == 1 {
			return roomOn("a")
		}
		return nil
	})
	err := r.svc.Tick(context.Background())
	if !errors.Is(err, errNoRoom) {
		t.Fatalf("tick = %v, want errNoRoom", err)
	}
	if got := len(r.claudeCalls()); got != 1 {
		t.Errorf("claude ran %d times, want 1 (only chunk 1)", got)
	}
	assertNothingStored(t, r)
}

func TestNoRoomBeforeTheCrossPassStoresNothingAndALaterTickReviewsFromScratch(t *testing.T) {
	r := crossRig(t, plainReplies, envelope("sx", "x", ""))
	roomPlan(r, func(k int) []Candidate {
		if k == 4 {
			return nil
		}
		return roomOn("a")
	})
	err := r.svc.Tick(context.Background())
	if !errors.Is(err, errNoRoom) {
		t.Fatalf("tick = %v, want errNoRoom", err)
	}
	if got := len(r.claudeCalls()); got != 3 {
		t.Errorf("claude ran %d times, want 3 (all chunks, no cross pass)", got)
	}
	assertNothingStored(t, r)

	// room is back: the whole PR is reviewed again from chunk 1
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatalf("second tick = %v", err)
	}
	if got := len(r.claudeCalls()); got != 7 {
		t.Errorf("claude ran %d times in total, want 7 (3 + 4)", got)
	}
	got, ok, _ := r.svc.Get(storedID)
	if !ok {
		t.Fatal("review not stored by the second tick")
	}
	if got.Error != "" {
		t.Errorf("error = %q, want a complete review", got.Error)
	}
}

func TestAnAccountAtNinetyFivePercentIsNoRoomForAMidReviewPick(t *testing.T) {
	r := crossRig(t, plainReplies, envelope("sx", "x", ""))
	roomPlan(r, func(k int) []Candidate {
		if k == 1 {
			return roomOn("a")
		}
		return []Candidate{{ID: "a", ConfigDir: "/a/a", FiveHourUsedPct: pct(95)}}
	})
	err := r.svc.Tick(context.Background())
	if !errors.Is(err, errNoRoom) {
		t.Fatalf("tick = %v, want errNoRoom", err)
	}
	if got := len(r.claudeCalls()); got != 1 {
		t.Errorf("claude ran %d times, want 1", got)
	}
	assertNothingStored(t, r)
}

func TestStoppedReviewCleansItsTempWorktree(t *testing.T) {
	r := crossRig(t, plainReplies, envelope("sx", "x", ""))
	roomPlan(r, func(k int) []Candidate {
		if k == 1 {
			return roomOn("a")
		}
		return nil
	})
	_ = r.svc.Tick(context.Background())
	ents, err := os.ReadDir(filepath.Join(r.dir, "work"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "wt-") {
			t.Errorf("temp worktree left behind: %s", e.Name())
		}
	}
	var adds, removes int
	for _, l := range r.fx.lines() {
		switch {
		case strings.HasPrefix(l, "git worktree add --detach "):
			adds++
		case strings.HasPrefix(l, "git worktree remove --force "):
			removes++
		}
	}
	if adds != 1 || removes != 1 {
		t.Errorf("worktree add %d remove %d, want 1 and 1", adds, removes)
	}
}

func TestTickStopsAtNoRoomMidReviewAndLeavesTheNextPRUntouched(t *testing.T) {
	r := crossRig(t, plainReplies, envelope("sx", "x", ""))
	// both PRs are asked for, #7 is the three chunk one
	both := `[{"number":7,"title":"Fix it","url":"https://x/7","isDraft":false,"author":{"login":"ann"},"repository":{"nameWithOwner":"o/r"}},{"number":8,"title":"Huge","url":"https://x/8","isDraft":false,"author":{"login":"bo"},"repository":{"nameWithOwner":"o/r"}}]`
	r.fx.rules = append([]fakeRule{{has: "search prs", out: both}}, r.fx.rules...)
	roomPlan(r, func(k int) []Candidate {
		if k == 1 {
			return roomOn("a")
		}
		return nil
	})
	err := r.svc.Tick(context.Background())
	if !errors.Is(err, errNoRoom) {
		t.Fatalf("tick = %v, want errNoRoom", err)
	}
	for _, l := range r.fx.lines() {
		if strings.Contains(l, "pr view 8") || strings.Contains(l, "pr diff 8") {
			t.Errorf("#8 was touched after errNoRoom: %s", l)
		}
	}
	if _, ok, _ := r.svc.Get("o-r-8-2222222"); ok {
		t.Error("#8 has a stored review")
	}
	assertNothingStored(t, r)
}
