package live

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

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

var epoch = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

// gated is a fetch that counts its calls and waits for a release
type gated struct {
	calls   atomic.Int32
	release chan struct{}
	val     atomic.Int32
	err     atomic.Value // error
}

func newGated() *gated { return &gated{release: make(chan struct{}, 16)} }

func (g *gated) fetch() (int, error) {
	g.calls.Add(1)
	<-g.release
	if e, _ := g.err.Load().(error); e != nil {
		return 0, e
	}
	return int(g.val.Load()), nil
}

func TestTheFirstGetWaitsForTheValue(t *testing.T) {
	g := newGated()
	g.val.Store(7)
	g.release <- struct{}{}
	c := newCache(g.fetch, time.Minute, (&fakeClock{t: epoch}).now)

	v, have, err := c.get(context.Background(), time.Second)

	if v != 7 || !have || err != nil {
		t.Errorf("get = %d, %v, %v", v, have, err)
	}
}

func TestAGetThatRunsOutOfGraceReturnsWithoutAValueAndTheFetchGoesOn(t *testing.T) {
	g := newGated()
	g.val.Store(7)
	c := newCache(g.fetch, time.Minute, (&fakeClock{t: epoch}).now)

	_, have, _ := c.get(context.Background(), 20*time.Millisecond)
	if have {
		t.Fatal("nothing was fetched yet")
	}
	g.release <- struct{}{}
	v, have, _ := c.get(context.Background(), time.Second)
	if v != 7 || !have || g.calls.Load() != 1 {
		t.Errorf("the same fetch finished: %d %v after %d calls", v, have, g.calls.Load())
	}
}

func TestAnOldValueIsReturnedAtOnceWhileOneRefreshRuns(t *testing.T) {
	clock := &fakeClock{t: epoch}
	g := newGated()
	g.val.Store(1)
	g.release <- struct{}{}
	c := newCache(g.fetch, time.Minute, clock.now)
	c.get(context.Background(), time.Second)

	clock.advance(2 * time.Minute)
	g.val.Store(2)
	for i := 0; i < 5; i++ {
		v, have, _ := c.get(context.Background(), time.Second)
		if v != 1 || !have {
			t.Fatalf("the old value is used while it is refreshed: %d %v", v, have)
		}
	}
	g.release <- struct{}{}
	waitUntil(t, func() bool { v, _, _ := c.get(context.Background(), 0); return v == 2 })
	if g.calls.Load() != 2 {
		t.Errorf("five gets share one refresh, calls = %d", g.calls.Load())
	}
}

func TestAnInvalidatedValueIsWaitedForAgain(t *testing.T) {
	g := newGated()
	g.val.Store(1)
	g.release <- struct{}{}
	c := newCache(g.fetch, time.Minute, (&fakeClock{t: epoch}).now)
	c.get(context.Background(), time.Second)

	c.invalidate()
	g.val.Store(2)
	g.release <- struct{}{}
	if v, _, _ := c.get(context.Background(), time.Second); v != 2 {
		t.Errorf("get after invalidate = %d, want the fresh 2", v)
	}
}

func TestAFailedFetchKeepsTheLastValueAndSaysWhy(t *testing.T) {
	clock := &fakeClock{t: epoch}
	g := newGated()
	g.val.Store(1)
	g.release <- struct{}{}
	c := newCache(g.fetch, time.Minute, clock.now)
	c.get(context.Background(), time.Second)

	clock.advance(2 * time.Minute)
	g.err.Store(errors.New("claude hung"))
	g.release <- struct{}{}
	c.get(context.Background(), time.Second)
	waitUntil(t, func() bool { _, _, err := c.get(context.Background(), 0); return err != nil })

	v, have, err := c.get(context.Background(), 0)
	if v != 1 || !have || err == nil || err.Error() != "claude hung" {
		t.Errorf("get = %d, %v, %v", v, have, err)
	}
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestASetDuringARefreshIsNotOverwrittenByIt(t *testing.T) {
	g := newGated()
	g.val.Store(1)
	g.release <- struct{}{}
	clock := &fakeClock{t: epoch}
	c := newCache(g.fetch, time.Minute, clock.now)
	c.get(context.Background(), time.Second)

	clock.advance(2 * time.Minute)
	c.get(context.Background(), 0) // an old refresh starts and blocks
	waitUntil(t, func() bool { return g.calls.Load() == 2 })
	c.set(9)
	g.val.Store(1)
	g.release <- struct{}{}
	waitUntil(t, func() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.running == nil })

	if v, _, _ := c.get(context.Background(), 0); v != 9 {
		t.Errorf("the answer that was set is gone: %d", v)
	}
}

func TestAnInvalidateDuringARefreshStillMakesTheNextGetWait(t *testing.T) {
	g := newGated()
	g.val.Store(1)
	g.release <- struct{}{}
	clock := &fakeClock{t: epoch}
	c := newCache(g.fetch, time.Minute, clock.now)
	c.get(context.Background(), time.Second)

	clock.advance(2 * time.Minute)
	c.get(context.Background(), 0)
	waitUntil(t, func() bool { return g.calls.Load() == 2 })
	c.invalidate()
	g.release <- struct{}{} // the old refresh ends and is discarded
	waitUntil(t, func() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.running == nil })

	g.val.Store(3)
	g.release <- struct{}{}
	if v, _, _ := c.get(context.Background(), time.Second); v != 3 || g.calls.Load() != 3 {
		t.Errorf("get = %d after %d fetches, want a new fetch's 3", v, g.calls.Load())
	}
}
