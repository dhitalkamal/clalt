package wrapper

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

// endWithoutMoving ends the session by itself and returns the notices that were
// printed once claude was gone.
func (r *rig) endWithoutMoving() string {
	r.t.Helper()
	n := len(r.launch.started())
	r.launch.proc(n - 1).end(nil)
	r.result()
	return r.noticeText()
}

func TestAMoveThatCannotBePreparedLeavesClaudeAloneAndSaysSoAtTheEnd(t *testing.T) {
	r := newRig(t)
	r.acc.prepare = func(id, cwd string) (Prepared, error) {
		return Prepared{}, errors.New("personal is not signed in. sign in with: clalt login personal")
	}
	r.run()
	r.request("personal")
	r.idle()
	waitFor(t, "the preparation", func() bool { return len(r.acc.calls()) == 1 })
	settle()

	if term, _ := r.launch.proc(0).counts(); term != 0 {
		t.Error("claude was ended for a move that could not be made")
	}
	if len(r.notes.all()) != 0 {
		t.Errorf("printed %v while claude owned the terminal", r.notes.all())
	}
	if HasRequest(r.reg.dir, handle) {
		t.Error("the request stays and fails again at every pause")
	}
	if got := r.endWithoutMoving(); !strings.Contains(got, "clalt login personal") || !strings.Contains(got, "work to personal") {
		t.Errorf("the notice at the end: %q", got)
	}
}

func TestTheRulesOfTheTargetAreCheckedForBothFolders(t *testing.T) {
	// real absolute paths: a rule is only an absolute path on every platform
	here, away := t.TempDir(), t.TempDir()
	r := newRig(t)
	r.sess.Cwd = here
	r.acc.cands[1].Folders = []string{here}
	r.run()
	r.request("personal")
	// the session has gone to another folder since it started
	r.clk.Advance(10 * time.Second)
	r.feed.set(handle, SessionInfo{State: Idle, Cwd: away, UpdatedAt: r.clk.Now().Add(-5 * time.Second)})
	settle()

	if term, _ := r.launch.proc(0).counts(); term != 0 || len(r.acc.calls()) != 0 {
		t.Error("a move across a folder rule was made, or prepared")
	}
	if got := r.endWithoutMoving(); !strings.Contains(got, "folder rules") || !strings.Contains(got, away) {
		t.Errorf("the notice: %q", got)
	}
}

func TestAMoveToTheAccountItIsOnIsDropped(t *testing.T) {
	r := newRig(t)
	r.run()
	r.request("work")
	r.idle()
	settle()

	if term, _ := r.launch.proc(0).counts(); term != 0 {
		t.Error("claude was restarted under the account it was on")
	}
	if got := r.endWithoutMoving(); !strings.Contains(got, "already") {
		t.Errorf("the notice: %q", got)
	}
}

func TestAMoveToAnAccountThatDoesNotExistIsDropped(t *testing.T) {
	r := newRig(t)
	r.run()
	r.request("nobody")
	r.idle()
	settle()

	if term, _ := r.launch.proc(0).counts(); term != 0 {
		t.Error("claude was ended")
	}
	if got := r.endWithoutMoving(); !strings.Contains(got, "nobody") {
		t.Errorf("the notice: %q", got)
	}
}

// no session id yet means claude has not told the registry, and --resume has
// nothing to resume. the request waits for it
func TestARequestWaitsForTheClaudeSessionId(t *testing.T) {
	r := newRig(t)
	r.reg.setSessionID("")
	r.run()
	r.request("personal")
	r.idle()
	settle()

	if term, _ := r.launch.proc(0).counts(); term != 0 || !HasRequest(r.reg.dir, handle) {
		t.Fatal("the move was made, or the request dropped, with no session to resume")
	}

	r.reg.setSessionID("sid-9")
	waitFor(t, "the move", func() bool { return len(r.launch.started()) == 2 })
	if got := r.launch.started()[1].args; got[1] != "sid-9" {
		t.Errorf("resumed %v", got)
	}
}

// the user can type the next prompt while the target is being prepared
func TestAPauseThatEndsWhileThePreparationRunsGivesTheRequestBack(t *testing.T) {
	r := newRig(t)
	r.acc.prepare = func(id, cwd string) (Prepared, error) {
		r.busy()
		return Prepared{Label: id, Folder: "/f/" + id}, nil
	}
	r.run()
	r.request("personal")
	r.idle()
	waitFor(t, "the preparation", func() bool { return len(r.acc.calls()) == 1 })
	settle()

	if term, _ := r.launch.proc(0).counts(); term != 0 {
		t.Fatal("claude was ended in the middle of a turn")
	}
	waitFor(t, "the request to be given back", func() bool { return HasRequest(r.reg.dir, handle) })

	r.acc.mu.Lock()
	r.acc.prepare = nil
	r.acc.mu.Unlock()
	r.idle()
	waitFor(t, "the move at the next pause", func() bool { return len(r.launch.started()) == 2 })
}

func TestAnAnswerThatCameLaterIsNotOverwrittenByTheRequestThatIsGivenBack(t *testing.T) {
	r := newRig(t)
	r.acc.prepare = func(id, cwd string) (Prepared, error) {
		r.busy()
		r.request("newer")
		return Prepared{Label: id, Folder: "/f/" + id}, nil
	}
	r.run()
	r.request("personal")
	r.idle()
	waitFor(t, "the preparation", func() bool { return len(r.acc.calls()) == 1 })
	settle()

	got, ok, err := ClaimRequest(r.reg.dir, handle)
	if err != nil || !ok || got.To != "newer" {
		t.Errorf("the request now is %+v, %v, %v, want the newer one", got, ok, err)
	}
}

func TestAStartThatFailsUnderTheTargetResumesUnderTheOldAccount(t *testing.T) {
	r := newRig(t)
	r.launch.fail["/f/personal"] = errors.New("claude is gone")
	r.run()
	r.request("personal")
	r.idle()

	waitFor(t, "the fallback start", func() bool { return len(r.launch.started()) == 3 })

	st := r.launch.started()
	if st[1].folder != "/f/personal" || st[2].folder != "/f/work" || st[2].args[0] != "--resume" || st[2].args[1] != "sid-1" {
		t.Errorf("starts = %+v, want personal tried, then a resume under work", st)
	}
	notice := r.noticeText()
	if !strings.Contains(notice, "claude is gone") || !strings.Contains(notice, "work") {
		t.Errorf("the notice: %q", notice)
	}
	for _, c := range r.reg.changes() {
		if c.Account == "personal" {
			t.Errorf("the registry was told the session moved: %+v", c)
		}
	}
}

func TestWhenNothingCanBeStartedTheErrorSaysHowToResumeByHand(t *testing.T) {
	r := newRig(t)
	r.launch.fail["/f/personal"] = errors.New("no")
	r.launch.spoil = func(p *fakeProc, n int) {}
	r.run()
	r.launch.fail["/f/work"] = errors.New("no either")
	r.request("personal")
	r.idle()

	err := r.result()

	if err == nil || !strings.Contains(err.Error(), "--resume sid-1") || !strings.Contains(err.Error(), "clalt run work") {
		t.Errorf("Run returned %v", err)
	}
}

// a claude that is still there after a kill must not be left running beside a
// new one in the same terminal
func TestAClaudeThatSurvivesAKillEndsTheWrapperWithoutAnotherClaude(t *testing.T) {
	r := newRig(t)
	r.launch.spoil = func(p *fakeProc, n int) { p.ignoreTerm, p.ignoreKill = true, true }
	r.cfg.KillWait = 40 * time.Millisecond
	r.run()
	r.request("personal")
	r.idle()

	err := r.result()

	if err == nil || !strings.Contains(err.Error(), "did not end") {
		t.Errorf("Run returned %v", err)
	}
	if n := len(r.launch.started()); n != 1 {
		t.Errorf("%d starts beside a claude that is still running", n)
	}
	if got := r.term.got(); !reflect.DeepEqual(got, []bool{true}) {
		t.Errorf("terminal restored %v, want once and forced", got)
	}
}

func TestTheTerminalIsRestoredWhenTheUserQuitsDuringAMove(t *testing.T) {
	r := newRig(t)
	r.launch.spoil = func(p *fakeProc, n int) { p.ignoreTerm = n == 0 }
	r.cfg.Grace = 400 * time.Millisecond
	r.run()
	r.request("personal")
	r.idle()
	waitFor(t, "the first claude to be told to end", func() bool { term, _ := r.launch.proc(0).counts(); return term == 1 })

	r.sigs <- syscall.SIGHUP
	r.result()

	if len(r.term.got()) != 1 {
		t.Errorf("terminal restored %v", r.term.got())
	}
}

func TestAMoveWhoseAccountsCannotBeReadIsNotMadeAndIsSaid(t *testing.T) {
	r := newRig(t)
	r.acc.candErr = errors.New("state.json is not readable")
	r.run()
	r.request("personal")
	r.idle()
	settle()

	if term, _ := r.launch.proc(0).counts(); term != 0 {
		t.Error("claude was ended without knowing where to")
	}
	if got := r.endWithoutMoving(); !strings.Contains(got, "state.json is not readable") {
		t.Errorf("the notice: %q", got)
	}
}

func TestAFailedRequestLeavesAResultForClaltMove(t *testing.T) {
	r := newRig(t)
	r.acc.prepare = func(id, cwd string) (Prepared, error) { return Prepared{}, errors.New("not signed in") }
	r.run()
	r.request("personal")
	r.idle()
	waitFor(t, "the result", func() bool {
		path, _ := RequestPath(r.reg.dir, handle)
		_, err := os.Stat(path + ".result")
		return err == nil
	})
	got, ok := TakeResult(r.reg.dir, handle)
	if !ok || got.To != "personal" || !strings.Contains(got.Error, "not signed in") {
		t.Errorf("result = %+v, %v", got, ok)
	}
}

// the process was already gone when it was asked to end: the user quit
func TestAClaudeThatIsGoneWhenToldToEndEndsTheWrapperAndBlocksNothing(t *testing.T) {
	r := newRig(t)
	r.launch.spoil = func(p *fakeProc, n int) { p.terminateErr = os.ErrProcessDone }
	r.acc.prepare = func(id, cwd string) (Prepared, error) {
		return Prepared{Label: id, Folder: "/f/" + id}, nil
	}
	r.run()
	r.request("personal")
	r.idle()
	waitFor(t, "the terminate", func() bool { return r.launch.proc(0).terminateCalls() == 1 })
	r.launch.proc(0).end(errors.New("exit status 0"))

	if err := r.result(); err == nil || !strings.Contains(err.Error(), "exit status 0") {
		t.Errorf("Run returned %v", err)
	}
	if n := len(r.launch.started()); n != 1 {
		t.Errorf("%d starts after the user quit", n)
	}
}

func resultOf(r *rig) Result {
	r.t.Helper()
	waitFor(r.t, "the result", func() bool {
		path, _ := RequestPath(r.reg.dir, handle)
		_, err := os.Stat(path + ".result")
		return err == nil
	})
	got, _ := TakeResult(r.reg.dir, handle)
	return got
}

func TestAnUnreadableAccountListLeavesAResult(t *testing.T) {
	r := newRig(t)
	r.acc.candErr = errors.New("state.json is not readable")
	r.run()
	r.request("personal")
	r.idle()
	if got := resultOf(r); !strings.Contains(got.Error, "state.json is not readable") {
		t.Errorf("result = %+v", got)
	}
}

func TestARequestForTheCurrentAccountLeavesAResult(t *testing.T) {
	r := newRig(t)
	r.run()
	r.request("work")
	r.idle()
	if got := resultOf(r); !strings.Contains(got.Error, "already") {
		t.Errorf("result = %+v", got)
	}
}

func TestARequestThatCannotBeReadLeavesAResult(t *testing.T) {
	r := newRig(t)
	r.run()
	path, _ := RequestPath(r.reg.dir, handle)
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.idle()
	if got := resultOf(r); got.Error == "" {
		t.Errorf("result = %+v", got)
	}
}

// the user typed /exit just before the pause was acted on
func TestAClaudeThatEndsOnItsOwnAfterTheSignalIsNotReplaced(t *testing.T) {
	r := newRig(t)
	r.launch.spoil = func(p *fakeProc, n int) { p.quitOnTerm = n == 0 }
	r.run()
	r.request("personal")
	r.idle()

	err := r.result()

	if n := len(r.launch.started()); n != 1 {
		t.Errorf("%d starts: the user quit and got a new claude", n)
	}
	if err != nil {
		t.Errorf("Run returned %v, want what claude ended with (nil)", err)
	}
}
