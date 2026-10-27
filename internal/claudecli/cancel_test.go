package claudecli

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"runtime"
	"testing"
	"time"
)

// the fake traps SIGINT, which only works if this process did not inherit it as
// ignored (a shell does that for background jobs).
func skipIfInterruptIgnored(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("windows cannot interrupt another process: a cancel waits out the grace and kills")
	}
	if signal.Ignored(os.Interrupt) {
		t.Skip("SIGINT is ignored here, so the fake cannot trap it")
	}
}

// patient gives a cancelled claude a minute to stop after the interrupt. the
// grace is only an upper bound, so a fake that exits at once is not slowed down,
// and a slow machine cannot turn a clean exit into a kill. only the tests that
// need the kill shorten it.
func patient(t *testing.T) {
	t.Helper()
	old := cancelGrace
	cancelGrace = time.Minute
	t.Cleanup(func() { cancelGrace = old })
}

// patience is how long a test waits for a cancelled call to come back: the grace
// period, which ends in a kill, plus room for a loaded machine. a call that
// stops by itself returns long before this.
func patience() time.Duration { return cancelGrace + 30*time.Second }

func waitForFile(t *testing.T, f *fake, name string) {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for !f.has(name) {
		if time.Now().After(deadline) {
			t.Fatalf("the fake never wrote %s", name)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// cancelledStatus starts Status against a hanging fake, cancels it once the
// fake is waiting, and returns how long Status took to return and its error.
func cancelledStatus(t *testing.T, f *fake) (time.Duration, error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := Status(ctx, t.TempDir())
		done <- err
	}()
	waitForFile(t, f, "ready")
	start := time.Now()
	cancel()
	select {
	case err := <-done:
		return time.Since(start), err
	case <-time.After(patience()):
		t.Fatalf("Status was still running %s after the cancel", patience())
		return 0, nil
	}
}

func TestCancelInterruptsClaudeInsteadOfKillingIt(t *testing.T) {
	skipIfInterruptIgnored(t)
	patient(t)
	f := newFake(t)
	f.hang("trap")

	_, err := cancelledStatus(t, f)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	// only a signal the script could catch leaves this behind; a kill cannot
	if !f.has("interrupted") {
		t.Error("claude never saw an interrupt, it was killed")
	}
}

func TestCancelLetsAnInFlightRefreshFinish(t *testing.T) {
	skipIfInterruptIgnored(t)
	patient(t)
	f := newFake(t)
	f.hang("slow")

	took, err := cancelledStatus(t, f)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if !f.has("finished") || took < 500*time.Millisecond {
		t.Errorf("claude was cut off: finished=%v after %s, want it to run its second", f.has("finished"), took)
	}
}

func TestCancelFallsBackToKillForAClaudeThatIgnoresTheInterrupt(t *testing.T) {
	skipIfInterruptIgnored(t)
	old := cancelGrace
	cancelGrace = 300 * time.Millisecond
	t.Cleanup(func() { cancelGrace = old })
	f := newFake(t)
	f.hang("ignore")

	took, err := cancelledStatus(t, f)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if took < cancelGrace {
		t.Errorf("returned after %s, before the %s grace was up", took, cancelGrace)
	}
}

func TestCancelledLoginCommandIsInterruptedToo(t *testing.T) {
	skipIfInterruptIgnored(t)
	patient(t)
	f := newFake(t)
	f.hang("trap")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd, err := Login(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitForFile(t, f, "ready")

	cancel()
	cmd.Wait()

	if !f.has("interrupted") {
		t.Error("claude never saw an interrupt, it was killed")
	}
}
