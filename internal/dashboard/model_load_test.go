package dashboard

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *fakeClock { return &fakeClock{t: time.Date(2025, 10, 15, 23, 53, 0, 0, time.UTC)} }

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func upd(m model, msg tea.Msg) (model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(model), cmd
}

// runAll runs a command and every member of a batch, each with a short wait
func runAll(cmd tea.Cmd) []tea.Msg {
	msg := RunCmd(cmd, 60*time.Millisecond)
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, runAll(c)...)
		}
		return out
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}

func clockOptions(c *fakeClock) Options {
	opt := testOptions()
	opt.Now = c.now
	opt.Refresh = 5 * time.Second
	return opt
}

func TestNewModelHasItsFirstLoadComing(t *testing.T) {
	m := newModel(context.Background(), &fakeSource{}, &fakeActions{}, testOptions())
	if !m.loading {
		t.Error("a new model counts its first load as in flight")
	}
	batch, ok := RunCmd(m.Init(), Patience).(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Errorf("Init batches the first load and the refresh tick, got %v", batch)
	}
}

func TestFirstScreenSaysLoadingUntilTheSnapshotArrives(t *testing.T) {
	m := newModel(context.Background(), &fakeSource{snap: fiveSnapshot()}, &fakeActions{}, testOptions())
	m, _ = upd(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	out := Screen(m)
	if !strings.Contains(out, "Loading") || strings.Contains(out, "morgan") {
		t.Errorf("before the first snapshot the panes say they are loading:\n%s", out)
	}
	m, _ = upd(m, RunCmd(m.loadCmd(), Patience))
	out = Screen(m)
	if strings.Contains(out, "Loading") || !strings.Contains(out, "morgan") || !strings.Contains(out, "5 accounts, 3 running") {
		t.Errorf("the snapshot replaces the loading text:\n%s", out)
	}
}

func TestLoadRunsInsideACommandNotTheUpdateLoop(t *testing.T) {
	src := &fakeSource{snap: fiveSnapshot()}
	m := newModel(context.Background(), src, &fakeActions{}, testOptions())
	m, _ = upd(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if src.calls.Load() != 0 {
		t.Fatal("building and resizing the model must not call the source")
	}
	cmd := m.loadCmd()
	if src.calls.Load() != 0 {
		t.Fatal("creating the command must not call the source either")
	}
	cmd()
	if src.calls.Load() != 1 {
		t.Error("running the command calls the source once")
	}
}

func TestTickWhileALoadIsInFlightStartsNothing(t *testing.T) {
	clock := newClock()
	src := &fakeSource{snap: fiveSnapshot()}
	m := newModel(context.Background(), src, &fakeActions{}, clockOptions(clock))
	clock.advance(time.Hour)
	for i := 0; i < 5; i++ {
		var cmd tea.Cmd
		m, cmd = upd(m, tickMsg{})
		runAll(cmd)
	}
	if src.calls.Load() != 0 || !m.loading {
		t.Errorf("the first load is still in flight, ticks must not start another (calls %d, loading %v)", src.calls.Load(), m.loading)
	}
}

func TestRefreshWaitsForTheIntervalThenLoadsOnce(t *testing.T) {
	clock := newClock()
	src := &fakeSource{snap: fiveSnapshot()}
	m := loadedModel(src, &fakeActions{}, clockOptions(clock), 100, 30)
	if src.calls.Load() != 1 {
		t.Fatalf("setup: one load so far, got %d", src.calls.Load())
	}
	clock.advance(4 * time.Second)
	var cmd tea.Cmd
	m, cmd = upd(m, tickMsg{})
	runAll(cmd)
	if src.calls.Load() != 1 || m.loading {
		t.Error("four seconds in, the five second refresh is not due")
	}
	clock.advance(time.Second)
	m, cmd = upd(m, tickMsg{})
	msgs := runAll(cmd)
	if src.calls.Load() != 2 || !m.loading {
		t.Errorf("at the interval a load starts (calls %d, loading %v)", src.calls.Load(), m.loading)
	}
	if len(msgs) == 0 {
		t.Error("the load reports back with a message")
	}
}

func TestASlowLoadNeverStacksUp(t *testing.T) {
	clock := newClock()
	src := &fakeSource{snap: fiveSnapshot()}
	m := loadedModel(src, &fakeActions{}, clockOptions(clock), 100, 30)
	src.gate = make(chan struct{})
	clock.advance(10 * time.Second)

	var cmd tea.Cmd
	m, cmd = upd(m, tickMsg{})
	batch, ok := RunCmd(cmd, Patience).(tea.BatchMsg)
	if !ok {
		t.Fatal("the tick command batches the refresh tick and the load")
	}
	for _, member := range batch {
		go member()
	}
	deadline := time.Now().Add(Patience)
	for src.calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if src.calls.Load() != 2 {
		t.Fatalf("the due tick should start the second load, calls %d", src.calls.Load())
	}
	for i := 0; i < 20; i++ {
		clock.advance(10 * time.Second)
		var c tea.Cmd
		m, c = upd(m, tickMsg{})
		runAll(c)
	}
	if got := src.calls.Load(); got != 2 {
		t.Errorf("one load in flight, twenty ticks later still %d calls", got)
	}
	close(src.gate)
}

func TestLoadResultClearsTheInFlightFlag(t *testing.T) {
	m := newModel(context.Background(), &fakeSource{}, &fakeActions{}, testOptions())
	m, _ = upd(m, loadedMsg{snap: fiveSnapshot()})
	if m.loading || !m.have {
		t.Errorf("a result ends the load (loading %v, have %v)", m.loading, m.have)
	}
	m.loading = true
	m, _ = upd(m, loadedMsg{err: errors.New("boom")})
	if m.loading {
		t.Error("an error ends the load too")
	}
}

func TestHungLoadEndsAsAnErrorNotAnEternalSpinner(t *testing.T) {
	src := &fakeSource{snap: fiveSnapshot(), gate: make(chan struct{})}
	opt := testOptions()
	opt.LoadTimeout = 20 * time.Millisecond
	m := newModel(context.Background(), src, &fakeActions{}, opt)
	m, _ = upd(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	msg := RunCmd(m.loadCmd(), Patience)
	loaded, ok := msg.(loadedMsg)
	if !ok || loaded.err == nil || loaded.err.Error() != "load timed out after 20ms" {
		t.Fatalf("a hung source should end with a timeout, got %#v", msg)
	}
	m, _ = upd(m, msg)
	if out := Screen(m); !strings.Contains(out, "load timed out after 20ms") {
		t.Errorf("the timeout shows in the dashboard:\n%s", out)
	}
}

func TestCancellingTheProgramContextStopsALoad(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	src := &fakeSource{snap: fiveSnapshot(), gate: make(chan struct{})}
	m := newModel(ctx, src, &fakeActions{}, testOptions())
	done := make(chan tea.Msg, 1)
	go func() { done <- m.loadCmd()() }()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case msg := <-done:
		if l, ok := msg.(loadedMsg); !ok || !errors.Is(l.err, context.Canceled) {
			t.Errorf("a canceled context ends the load with its error, got %#v", msg)
		}
	case <-time.After(Patience):
		t.Fatal("the load did not stop when the context was canceled")
	}
}
