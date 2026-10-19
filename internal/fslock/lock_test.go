package fslock

import (
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestAcquireIsMutuallyExclusive proves the lock actually excludes concurrent
// holders: many goroutines, each opening its OWN handle via Acquire (as
// separate processes would each open their own fd), race to bump a counter
// inside the critical section. If the lock ever let two holders in at once,
// the counter would exceed 1 concurrently.
func TestAcquireIsMutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.lock")

	var inside int32
	var maxSeen int32
	var wg sync.WaitGroup
	const n = 20
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l, err := Acquire(path, 10*time.Second)
			if err != nil {
				t.Errorf("Acquire: %v", err)
				return
			}
			cur := atomic.AddInt32(&inside, 1)
			for {
				old := atomic.LoadInt32(&maxSeen)
				if cur <= old || atomic.CompareAndSwapInt32(&maxSeen, old, cur) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			atomic.AddInt32(&inside, -1)
			if err := l.Release(); err != nil {
				t.Errorf("Release: %v", err)
			}
		}()
	}
	wg.Wait()
	if maxSeen != 1 {
		t.Errorf("max concurrent holders = %d, want 1", maxSeen)
	}
}

// TestAcquireTimesOut proves a bounded wait: a second handle that can never
// get the lock gives up with a clear error instead of hanging forever.
func TestAcquireTimesOut(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.lock")

	holder, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatalf("Acquire (holder): %v", err)
	}
	defer holder.Release()

	start := time.Now()
	_, err = Acquire(path, 200*time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("Acquire should have timed out while the lock is held")
	}
	if elapsed > 2*time.Second {
		t.Errorf("Acquire took %s to time out, want close to the 200ms budget", elapsed)
	}
}

// TestReleaseAllowsReacquire proves Release actually frees the lock for the
// next handle, rather than merely closing the fd without unlocking.
func TestReleaseAllowsReacquire(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.lock")

	l1, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := l1.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}

	l2, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatalf("second Acquire after release: %v", err)
	}
	_ = l2.Release()
}

// TestWithLockReleasesEvenOnError proves WithLock always releases the lock,
// so a caller's failure never wedges every future caller.
func TestWithLockReleasesEvenOnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.lock")

	sentinel := errFor(t)
	if err := WithLock(path, time.Second, func() error { return sentinel }); err != sentinel {
		t.Fatalf("WithLock error = %v, want sentinel", err)
	}

	// the lock must be free again
	l, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatalf("Acquire after failed WithLock: %v", err)
	}
	_ = l.Release()
}

func errFor(t *testing.T) error {
	t.Helper()
	return &testErr{"boom"}
}

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }
