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

func TestClaudeEndingByItselfEndsTheWrapperWithItsResult(t *testing.T) {
	r := newRig(t)
	r.run()
	boom := errors.New("exit status 3")

	r.launch.proc(0).end(boom)

	if err := r.result(); err != boom {
		t.Errorf("Run returned %v, want what claude ended with", err)
	}
	if got := r.reg.changes(); len(got) != 1 || got[0].ClaudePID != r.launch.proc(0).PID() {
		t.Errorf("the registry was told %+v, want the pid of the first claude", got)
	}
	if st := r.launch.started(); len(st) != 1 || st[0].folder != "/f/work" || !reflect.DeepEqual(st[0].args, r.sess.Args) {
		t.Errorf("starts = %+v, want the session's own folder and arguments", st)
	}
}

func TestACleanExitIsNil(t *testing.T) {
	r := newRig(t)
	r.run()
	r.launch.proc(0).end(nil)
	if err := r.result(); err != nil {
		t.Errorf("Run returned %v", err)
	}
}

func TestAStartThatFailsIsReturned(t *testing.T) {
	r := newRig(t)
	r.launch.fail["/f/work"] = errors.New("no claude here")

	err := Run(r.sess, r.deps())

	if err == nil || !strings.Contains(err.Error(), "no claude here") {
		t.Errorf("Run returned %v", err)
	}
}

func TestARequestMovesTheSessionAtTheNextPause(t *testing.T) {
	r := newRig(t)
	r.run()
	r.request("personal")
	r.idle()

	waitFor(t, "the second claude", func() bool { return len(r.launch.started()) == 2 })

	first := r.launch.proc(0)
	if term, kill := first.counts(); term != 1 || kill != 0 {
		t.Errorf("first claude: terminated %d, killed %d, want one SIGTERM and no kill", term, kill)
	}
	want := start{folder: "/f/personal", args: []string{"--resume", "sid-1", "--add-dir", "../a"}}
	if got := r.launch.started()[1]; !reflect.DeepEqual(got, want) {
		t.Errorf("second start = %+v, want %+v", got, want)
	}
	if got := r.term.got(); !reflect.DeepEqual(got, []bool{false}) {
		t.Errorf("terminal restored %v, want once and not forced", got)
	}
	notice := r.noticeText()
	for _, want := range []string{"3fa9c1d2", "from work to personal", "requested with clalt move"} {
		if !strings.Contains(notice, want) {
			t.Errorf("the notice %q lacks %q", notice, want)
		}
	}
	ch := r.reg.changes()
	if last := ch[len(ch)-1]; last.Account != "personal" || last.Folder != "/f/personal" || last.ClaudePID != r.launch.proc(1).PID() {
		t.Errorf("the registry was told %+v", last)
	}
	if HasRequest(r.reg.dir, handle) {
		t.Error("the request is still there after the move")
	}

	r.launch.proc(1).end(errors.New("exit status 0"))
	if err := r.result(); err == nil || !strings.Contains(err.Error(), "exit status 0") {
		t.Errorf("Run returned %v, want the result of the second claude", err)
	}
}

func TestNothingMovesWhileTheSessionIsBusy(t *testing.T) {
	r := newRig(t)
	r.run()
	r.request("personal")
	r.busy()
	settle()

	if n := len(r.launch.started()); n != 1 {
		t.Fatalf("%d starts while busy", n)
	}
	if !HasRequest(r.reg.dir, handle) {
		t.Error("the request was taken before there was a pause")
	}

	r.idle()
	waitFor(t, "the move at the pause", func() bool { return len(r.launch.started()) == 2 })
}

func TestOnlyAnIdleSessionIsAtAPause(t *testing.T) {
	for name, state := range map[string]State{"starting": Starting, "needs you": NeedsYou, "ended": Ended, "unknown": Unknown} {
		r := newRig(t)
		r.run()
		r.request("personal")
		r.clk.Advance(10 * time.Second)
		r.feed.set(handle, SessionInfo{State: state, UpdatedAt: r.clk.Now().Add(-5 * time.Second)})

		settle()

		if n := len(r.launch.started()); n != 1 {
			t.Errorf("%s: moved", name)
		}
		r.launch.proc(0).end(nil)
		r.result()
	}
}

func TestAnIdleThatIsTooFreshOrFromBeforeTheStartIsNotAPause(t *testing.T) {
	r := newRig(t)
	r.run()
	r.request("personal")

	// idle for half a second: the next prompt may be on its way
	r.clk.Advance(10 * time.Second)
	r.feed.set(handle, SessionInfo{State: Idle, UpdatedAt: r.clk.Now().Add(-500 * time.Millisecond)})
	settle()
	if n := len(r.launch.started()); n != 1 {
		t.Fatal("moved on an idle that had just begun")
	}
	// an idle that was written before this claude started is a leftover
	r.feed.set(handle, SessionInfo{State: Idle, UpdatedAt: clock.Add(-time.Hour)})
	settle()
	if n := len(r.launch.started()); n != 1 {
		t.Fatal("moved on the idle of a claude that is gone")
	}
	// and with no state at all nothing is known
	r.feed.mu.Lock()
	delete(r.feed.sessions, handle)
	r.feed.mu.Unlock()
	settle()
	if n := len(r.launch.started()); n != 1 {
		t.Fatal("moved with no feed state")
	}
}

func TestAClaudeThatIgnoresSIGTERMIsKilledAfterTheGraceAndTheTerminalIsForced(t *testing.T) {
	r := newRig(t)
	r.launch.spoil = func(p *fakeProc, n int) { p.ignoreTerm = n == 0 }
	r.run()
	r.request("personal")
	r.idle()

	waitFor(t, "the second claude", func() bool { return len(r.launch.started()) == 2 })

	if term, kill := r.launch.proc(0).counts(); term != 1 || kill != 1 {
		t.Errorf("terminated %d, killed %d, want one of each", term, kill)
	}
	if got := r.term.got(); !reflect.DeepEqual(got, []bool{true}) {
		t.Errorf("terminal restored %v, want once and forced", got)
	}
}

func TestSignalsAreRelayedToClaudeExceptTheInterrupt(t *testing.T) {
	r := newRig(t)
	r.run()

	r.sigs <- os.Interrupt
	r.sigs <- syscall.SIGTERM
	r.sigs <- syscall.SIGHUP

	waitFor(t, "the relay", func() bool { return len(r.launch.proc(0).got()) == 2 })
	if got := r.launch.proc(0).got(); !reflect.DeepEqual(got, []os.Signal{syscall.SIGTERM, syscall.SIGHUP}) {
		t.Errorf("relayed %v, want TERM and HUP: a Ctrl-C reaches claude by itself", got)
	}
}

func TestASignalAfterAMoveGoesToTheNewClaude(t *testing.T) {
	r := newRig(t)
	r.run()
	r.request("personal")
	r.idle()
	waitFor(t, "the second claude", func() bool { return len(r.launch.started()) == 2 })

	r.sigs <- syscall.SIGTERM

	waitFor(t, "the relay", func() bool { return len(r.launch.proc(1).got()) == 1 })
	if len(r.launch.proc(0).got()) != 0 {
		t.Error("the signal went to the claude that was gone")
	}
}

func TestAQuitSignalDuringAMoveEndsTheWrapperWithoutAnotherClaude(t *testing.T) {
	r := newRig(t)
	r.launch.spoil = func(p *fakeProc, n int) { p.ignoreTerm = n == 0 }
	r.cfg.Grace = 400 * time.Millisecond
	r.run()
	r.request("personal")
	r.idle()
	waitFor(t, "the first claude to be told to end", func() bool { term, _ := r.launch.proc(0).counts(); return term == 1 })

	r.sigs <- syscall.SIGHUP
	err := r.result()

	if n := len(r.launch.started()); n != 1 {
		t.Errorf("%d starts: a second claude was started after the user quit", n)
	}
	if err == nil {
		t.Error("Run returned nil after being killed by a signal")
	}
}

func TestClaudeEndingOnItsOwnWhileAMoveIsPreparedEndsTheWrapper(t *testing.T) {
	r := newRig(t)
	r.acc.prepare = func(id, cwd string) (Prepared, error) {
		r.launch.proc(0).end(errors.New("exit status 0"))
		time.Sleep(20 * time.Millisecond)
		return Prepared{Label: id, Folder: "/f/" + id}, nil
	}
	r.run()
	r.request("personal")
	r.idle()

	if err := r.result(); err == nil || !strings.Contains(err.Error(), "exit status 0") {
		t.Errorf("Run returned %v", err)
	}
	if n := len(r.launch.started()); n != 1 {
		t.Errorf("%d starts: the user had quit", n)
	}
}

func TestWhatIsLeftOfRequestsIsRemovedWhenTheWrapperEnds(t *testing.T) {
	r := newRig(t)
	r.run()
	r.request("personal")
	path, _ := RequestPath(r.reg.dir, handle)
	if err := os.WriteFile(path+".claimed", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	r.launch.proc(0).end(nil)
	r.result()

	if left, _ := os.ReadDir(r.reg.dir); len(left) != 0 {
		t.Errorf("left %v", left)
	}
}

func TestARegistryThatCannotBeUpdatedIsSaidAtTheEnd(t *testing.T) {
	r := newRig(t)
	r.reg.updateErr = errors.New("disk full")
	r.run()

	r.launch.proc(0).end(nil)
	r.result()

	if got := r.noticeText(); !strings.Contains(got, "session record") || !strings.Contains(got, "disk full") {
		t.Errorf("the notices: %q", got)
	}
}
