package live

import (
	"context"
	"sync"
	"time"
)

// cache keeps one value fresh in the background. get hands out the last good
// value at once and starts at most one refresh when it is old. a value that is
// missing or was invalidated is waited for, but only for the grace the caller
// gives, so a slow source cannot hold a load. the fetch never sees the
// caller's context: it may be an auth call that must not be cut short.
type cache[T any] struct {
	fetch func() (T, error)
	ttl   time.Duration
	now   func() time.Time

	mu      sync.Mutex
	val     T
	have    bool
	err     error
	at      time.Time
	stale   bool
	gen     int           // bumped by set and invalidate, so an older fetch cannot undo them
	running chan struct{} // closed when the refresh in flight ends
}

func newCache[T any](fetch func() (T, error), ttl time.Duration, now func() time.Time) *cache[T] {
	return &cache[T]{fetch: fetch, ttl: ttl, now: now}
}

// get returns the last good value, whether there is one, and the error of the
// last fetch (nil when it worked)
func (c *cache[T]) get(ctx context.Context, grace time.Duration) (T, bool, error) {
	c.mu.Lock()
	usable := c.have && !c.stale
	if !usable || c.now().Sub(c.at) >= c.ttl {
		c.startLocked()
	}
	wait := c.running
	if usable || wait == nil {
		defer c.mu.Unlock()
		return c.val, c.have, c.err
	}
	c.mu.Unlock()

	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-wait:
	case <-timer.C:
	case <-ctx.Done():
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.val, c.have, c.err
}

func (c *cache[T]) startLocked() {
	if c.running != nil {
		return
	}
	done := make(chan struct{})
	c.running = done
	gen := c.gen
	go func() {
		v, err := c.fetch()
		c.mu.Lock()
		// a set or an invalidate during the fetch is newer than its answer
		if c.gen == gen {
			c.at, c.err = c.now(), err
			if err == nil {
				c.val, c.have, c.stale = v, true, false
			}
		}
		c.running = nil
		c.mu.Unlock()
		close(done)
	}()
}

// invalidate says the value cannot be trusted any more, so the next get waits
// for a fresh one
func (c *cache[T]) invalidate() {
	c.mu.Lock()
	c.stale = true
	c.gen++
	c.mu.Unlock()
}

// set replaces the value with one that is known to be right
func (c *cache[T]) set(v T) {
	c.mu.Lock()
	c.val, c.have, c.err, c.stale, c.at = v, true, nil, false, c.now()
	c.gen++
	c.mu.Unlock()
}
