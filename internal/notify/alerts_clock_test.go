package notify

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

// an Alerts built by hand, without a clock, uses the real one
func TestAlertsWithoutAClockStillRateLimit(t *testing.T) {
	a, _ := newAlerts(t)
	a.Now = nil

	first, err1 := a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "work"})
	second, err2 := a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "work"})

	if err1 != nil || err2 != nil || first != Sent || second != AlreadyNotified {
		t.Errorf("got %v %v, %v %v", first, err1, second, err2)
	}
}

func TestAWhitespaceReasonCountsAsNone(t *testing.T) {
	a, f := newAlerts(t)

	a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "work", Reason: " \n\t "})

	calls := f.Calls()
	if len(calls) != 1 || calls[0].Args[len(calls[0].Args)-1] != "waiting for you" {
		t.Errorf("calls = %v", calls)
	}
}

// several hooks finding the same stale marker must still make one notification
func TestStaleMarkerTakeoverNotifiesOnce(t *testing.T) {
	for round := 0; round < 30; round++ {
		a, f := newAlerts(t)
		if _, err := a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "work"}); err != nil {
			t.Fatal(err)
		}
		old := noon.Add(-7 * time.Hour)
		if err := os.Chtimes(marker(a, "h1"), old, old); err != nil {
			t.Fatal(err)
		}
		before := len(f.Calls())
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "work"})
			}()
		}
		wg.Wait()
		if n := len(f.Calls()) - before; n != 1 {
			t.Fatalf("round %d: %d notifications for one wait", round, n)
		}
	}
}
