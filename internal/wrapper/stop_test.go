package wrapper

import (
	"os"
	"testing"
	"time"
)

func TestWriteAndClaimAStop(t *testing.T) {
	dir := t.TempDir()
	if HasStop(dir, handle) {
		t.Fatal("a stop before any was written")
	}
	if err := WriteStop(dir, handle, at); err != nil {
		t.Fatal(err)
	}
	if !HasStop(dir, handle) {
		t.Error("HasStop is false for a stop that was written")
	}
	if !ClaimStop(dir, handle) || ClaimStop(dir, handle) {
		t.Error("a stop must be claimed exactly once")
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Errorf("left %v behind", left)
	}
}

func TestAStopForABadHandleIsRefused(t *testing.T) {
	for _, h := range []string{"", "../x", "ext-1234", "ABCDEF0123456789"} {
		if err := WriteStop(t.TempDir(), h, at); err == nil {
			t.Errorf("WriteStop accepted %q", h)
		}
	}
}

func TestRemoveRequestsAlsoClearsAStop(t *testing.T) {
	dir := t.TempDir()
	_ = WriteStop(dir, handle, at)
	RemoveRequests(dir, handle)
	if HasStop(dir, handle) {
		t.Error("the stop outlived the session")
	}
}

func TestAStopEndsClaudeAtAPauseAndTheWrapperEndsCleanly(t *testing.T) {
	r := newRig(t)
	r.run()
	if err := WriteStop(r.reg.dir, handle, at); err != nil {
		t.Fatal(err)
	}
	r.idle()
	if err := r.result(); err != nil {
		t.Errorf("a stop is not an error, got %v", err)
	}
	if term, killed := r.launch.proc(0).counts(); term != 1 || killed != 0 {
		t.Errorf("terminated %d, killed %d", term, killed)
	}
	if n := len(r.launch.started()); n != 1 {
		t.Errorf("claude was started %d times, a stop must not resume", n)
	}
	if got := r.term.got(); len(got) != 1 || got[0] {
		t.Errorf("terminal restore calls %v", got)
	}
}

func TestAStopWaitsWhileClaudeIsBusy(t *testing.T) {
	r := newRig(t)
	r.run()
	r.busy()
	_ = WriteStop(r.reg.dir, handle, at)
	settle()
	if term, _ := r.launch.proc(0).counts(); term != 0 {
		t.Fatal("claude was ended in the middle of a step")
	}
	if !HasStop(r.reg.dir, handle) {
		t.Error("the stop was dropped before it could be carried out")
	}
	r.idle()
	if err := r.result(); err != nil {
		t.Errorf("got %v", err)
	}
}

func TestAStopEndsAClaudeThatWaitsForTheUser(t *testing.T) {
	r := newRig(t)
	r.run()
	r.clk.Advance(10 * time.Second)
	r.feed.set(handle, SessionInfo{State: NeedsYou, UpdatedAt: r.clk.Now()})
	_ = WriteStop(r.reg.dir, handle, at)
	if err := r.result(); err != nil {
		t.Errorf("got %v", err)
	}
}
