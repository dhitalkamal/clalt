package notify

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/fslock"
)

// a stale marker whose lock is held by someone else must not hold the hook for
// the old 5 seconds: the notification is skipped
func TestStaleMarkerWithABusyLockIsSkippedQuickly(t *testing.T) {
	a, f := newAlerts(t)
	if _, err := a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "work"}); err != nil {
		t.Fatal(err)
	}
	old := noon.Add(-7 * time.Hour)
	if err := os.Chtimes(marker(a, "h1"), old, old); err != nil {
		t.Fatal(err)
	}
	l, err := fslock.Acquire(filepath.Join(a.Dir, "waiting", ".lock-h1"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()
	before := len(f.Calls())

	start := time.Now()
	_, err = a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "work"})

	if err == nil || time.Since(start) > time.Second || len(f.Calls()) != before {
		t.Errorf("err %v after %v, %d new calls", err, time.Since(start), len(f.Calls())-before)
	}
}

func TestStaleMarkerLockWaitFollowsTheContext(t *testing.T) {
	a, _ := newAlerts(t)
	a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "work"})
	old := noon.Add(-7 * time.Hour)
	os.Chtimes(marker(a, "h1"), old, old)
	l, err := fslock.Acquire(filepath.Join(a.Dir, "waiting", ".lock-h1"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	a.NeedsYou(ctx, Event{Handle: "h1", Account: "work"})

	if time.Since(start) > 200*time.Millisecond {
		t.Errorf("waited %v on a 50ms context", time.Since(start))
	}
}
