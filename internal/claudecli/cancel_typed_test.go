package claudecli

import (
	"errors"
	"testing"
	"time"
)

func TestACancelledCallReturnsATypedErrorThatSaysHowItEnded(t *testing.T) {
	skipIfInterruptIgnored(t)
	old := cancelGrace
	t.Cleanup(func() { cancelGrace = old })
	for _, c := range []struct {
		mode   string
		grace  time.Duration
		killed bool
	}{
		// these stop by themselves, so the grace is only an upper bound and it can be
		// long: a loaded machine must not make it look like a kill
		{"trap", time.Minute, false},
		// finishes its work in a second, well inside the grace
		{"slow", time.Minute, false},
		// has to be killed, so its grace is short
		{"ignore", 400 * time.Millisecond, true},
	} {
		cancelGrace = c.grace
		f := newFake(t)
		f.hang(c.mode)

		_, err := cancelledStatus(t, f)

		var ce *CancelError
		if !errors.As(err, &ce) || ce.Killed != c.killed {
			t.Errorf("%s: err = %v, want a CancelError with Killed=%v", c.mode, err, c.killed)
		}
	}
}

func TestWithoutInterruptsTheGracePeriodDecidesWhenClaudeIsKilled(t *testing.T) {
	skipIfInterruptIgnored(t)
	old, oldOS := cancelGrace, goos
	cancelGrace = 400 * time.Millisecond
	goos = "windows"
	t.Cleanup(func() { cancelGrace, goos = old, oldOS })
	f := newFake(t)
	f.hang("trap")

	took, err := cancelledStatus(t, f)

	var ce *CancelError
	if !errors.As(err, &ce) || !ce.Killed {
		t.Errorf("err = %v, want a CancelError with Killed=true", err)
	}
	if f.has("interrupted") {
		t.Error("an interrupt was sent where the platform cannot send one")
	}
	if took < cancelGrace {
		t.Errorf("claude was killed after %s, before the %s grace was up", took, cancelGrace)
	}
}
