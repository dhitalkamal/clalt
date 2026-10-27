package dashboard

import (
	"context"
	"errors"
	"testing"
)

type ctxKey struct{}

// every action gets the context the program runs under, so an adapter can stop
// when the dashboard quits
func TestEveryActionGetsTheProgramContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxKey{}, "program")
	act := &fakeActions{cmd: helperCmd(0)}
	m := loadedModelIn(ctx, &fakeSource{snap: fiveSnapshot()}, act, testOptions(), 100, 30)
	m = pressSettleNoExec(m, "enter")
	m = pressSettleNoExec(m, "s")
	m = pressSettle(m, "e", "enter", "enter")
	m = pressSettle(m, "r", "x", "enter")
	m = pressSettle(m, "d", "y")
	m = pressSettle(m, "2", "x", "y")
	got := act.contexts()
	if len(got) != 6 {
		t.Fatalf("run, sign in, budget, rename, sign out and stop were all asked for: %v", act.log())
	}
	for i, c := range got {
		if c.Value(ctxKey{}) != "program" {
			t.Errorf("call %d (%s) got a context that is not the program's", i, act.log()[i])
		}
	}
}

// pressSettleNoExec presses keys and runs only the command that asks the
// actions, leaving the process it returns unrun
func pressSettleNoExec(m model, keys ...string) model {
	next, cmd := Press(m, keys...)
	if msg, ok := RunCmd(cmd, Patience).(launchMsg); ok {
		got, _ := upd(next.(model), launchMsg{err: msg.err})
		return got
	}
	return next.(model)
}

// when Load returns both a snapshot and an error, the error wins: the dashboard
// keeps showing the last good read
func TestAnErrorWinsOverTheSnapshotItComesWith(t *testing.T) {
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, testOptions(), 100, 30)
	partial := Snapshot{Accounts: fiveSnapshot().Accounts[:1]}
	next, _ := upd(m, loadedMsg{snap: partial, err: errors.New("half a read")})
	if len(next.snap.Accounts) != 5 || next.loadErr == nil {
		t.Errorf("the partial snapshot is ignored (%d accounts) and the error shows (%v)", len(next.snap.Accounts), next.loadErr)
	}
}

// actions run while a load is in flight, so adapters must be safe for
// concurrent use with Load
func TestAnActionRunsWhileALoadIsInFlight(t *testing.T) {
	src := &fakeSource{snap: fiveSnapshot()}
	act := &fakeActions{err: errors.New("no")}
	m := loadedModel(src, act, testOptions(), 100, 30)
	gate := make(chan struct{})
	src.mu.Lock()
	src.gate = gate
	src.mu.Unlock()
	m, load := m.startLoad()
	loaded := make(chan struct{})
	go func() { RunCmd(load, Patience); close(loaded) }()
	waitFor(t, "the second load to start", func() bool { return src.calls.Load() == 2 })

	pressSettle(m, "enter")
	if got := act.log(); len(got) != 1 {
		t.Errorf("the action was not held back by the load: %v", got)
	}
	select {
	case <-loaded:
		t.Fatal("the load should still be waiting on its gate")
	default:
	}
	close(gate)
	<-loaded
}
