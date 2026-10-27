package dashboard

import (
	"context"
	"testing"
	"time"
)

// deafSource ignores its context, like an adapter blocked in a call that
// cannot be cancelled
type deafSource struct{ release chan struct{} }

func (d deafSource) Load(context.Context) (Snapshot, error) {
	<-d.release
	return fiveSnapshot(), nil
}

// lateSource notices the context ended and hands back what it has, with no error
type lateSource struct{}

func (lateSource) Load(ctx context.Context) (Snapshot, error) {
	<-ctx.Done()
	return fiveSnapshot(), nil
}

func shortLimit(src Source) model {
	opt := testOptions()
	opt.LoadTimeout = 30 * time.Millisecond
	return newModel(context.Background(), src, &fakeActions{}, opt)
}

func TestALoadThatIgnoresItsContextStillTimesOut(t *testing.T) {
	d := deafSource{release: make(chan struct{})}
	defer close(d.release)
	l, ok := RunCmd(shortLimit(d).loadCmd(), Patience).(loadedMsg)
	if !ok || l.err == nil || l.err.Error() != "load timed out after 30ms" || len(l.snap.Accounts) != 0 {
		t.Errorf("the dashboard stops waiting on its own: %#v", l)
	}
}

func TestANilErrorAfterTheContextEndedIsATimeout(t *testing.T) {
	l, ok := RunCmd(shortLimit(lateSource{}).loadCmd(), Patience).(loadedMsg)
	if !ok || l.err == nil || l.err.Error() != "load timed out after 30ms" {
		t.Errorf("a read that finished after the deadline may be partial: %#v", l)
	}
}

func TestALoadInTimeIsNotATimeout(t *testing.T) {
	m := shortLimit(&fakeSource{snap: fiveSnapshot()})
	l, ok := RunCmd(m.loadCmd(), Patience).(loadedMsg)
	if !ok || l.err != nil || len(l.snap.Accounts) != 5 {
		t.Errorf("a quick source loads: %#v", l)
	}
}

func TestTheTimeoutNamesTheLimit(t *testing.T) {
	if got := timeoutError(30 * time.Second).Error(); got != "load timed out after 30s" {
		t.Errorf("the default limit reads %q", got)
	}
	if got := timeoutError(90 * time.Second).Error(); got != "load timed out after 1m30s" {
		t.Errorf("a longer limit reads %q", got)
	}
}

// the data is as old as the moment the read began, not the moment it got back
func TestAReadIsStampedWhereItStarted(t *testing.T) {
	clock := newClock()
	src := &fakeSource{snap: fiveSnapshot(), gate: make(chan struct{})}
	m := newModel(context.Background(), src, &fakeActions{}, clockOptions(clock))
	start := clock.now()
	done := make(chan loadedMsg, 1)
	go func() { done <- m.loadCmd()().(loadedMsg) }()
	waitFor(t, "the read to start", func() bool { return src.calls.Load() == 1 })
	clock.advance(7 * time.Second)
	close(src.gate)
	l := <-done
	if !l.at.Equal(start) {
		t.Errorf("the message carries the start of the read: %v, started %v", l.at, start)
	}
	m, _ = upd(m, l)
	if !m.loadedAt.Equal(start) {
		t.Errorf("the model keeps it: %v", m.loadedAt)
	}
}
