package claudecli

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestACallersDeadlineIsKeptAndOtherwiseAGenerousOneIsAdded(t *testing.T) {
	own, cancelOwn := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelOwn()
	got, cancel := withDefaultTimeout(own)
	defer cancel()
	want, _ := own.Deadline()
	if d, ok := got.Deadline(); !ok || !d.Equal(want) {
		t.Errorf("deadline = %v, want the caller's %v", d, want)
	}

	got, cancel = withDefaultTimeout(context.Background())
	defer cancel()
	d, ok := got.Deadline()
	if !ok || time.Until(d) < time.Minute || time.Until(d) > authTimeout+time.Second {
		t.Errorf("deadline = %v (%v), want about %s from now", d, ok, authTimeout)
	}
	if authTimeout != 2*time.Minute {
		t.Errorf("authTimeout = %s, want 2 minutes", authTimeout)
	}
}

// shortTimeout shortens the default timeout once the version check of the fake
// is done. the check runs inside the same timeout, and how long a new script
// takes to start is up to the machine, so it must not eat the short one.
func shortTimeout(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := ensureVersion(ctx); err != nil {
		t.Fatal(err)
	}
	old := authTimeout
	authTimeout = 400 * time.Millisecond
	t.Cleanup(func() { authTimeout = old })
}

func TestStatusAndLogoutStopWaitingAfterTheDefaultTimeout(t *testing.T) {
	skipIfInterruptIgnored(t)
	// the fake hangs for a while by itself, so only the deadline can end the call
	// with this error, whatever the speed of the machine
	f := newFake(t)
	f.hangOn("status", "trap")
	shortTimeout(t)
	if _, err := Status(context.Background(), t.TempDir()); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Status err = %v, want the deadline", err)
	}

	f2 := newFake(t)
	f2.hangOn("logout", "trap")
	shortTimeout(t)
	err := Logout(context.Background(), t.TempDir())
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, ErrSignOutUnknown) {
		t.Errorf("Logout err = %v, want the deadline and an unknown sign-out", err)
	}
}

func TestVersionStopsWaitingAfterTheDefaultTimeout(t *testing.T) {
	skipIfInterruptIgnored(t)
	old := authTimeout
	authTimeout = 400 * time.Millisecond
	t.Cleanup(func() { authTimeout = old })
	f := newFake(t)
	f.hangOn("version", "trap")

	if _, err := Version(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Version err = %v, want the deadline", err)
	}
}
