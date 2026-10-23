package claudecli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"testing"
	"time"
)

// exited runs a shell that ends the way asked and returns the error exec gives
// for it, since an *exec.ExitError cannot be made by hand.
func exited(t *testing.T, script string) *exec.ExitError {
	t.Helper()
	skipOnWindows(t)
	var ee *exec.ExitError
	if !errors.As(exec.Command("sh", "-c", script).Run(), &ee) {
		t.Fatalf("%q did not end with an exit error", script)
	}
	return ee
}

func sentAgo(d time.Duration) *cancelState {
	st := &cancelState{}
	st.at.Store(time.Now().Add(-d).UnixNano())
	return st
}

func TestAResultFromBeforeTheContextEndedIsKept(t *testing.T) {
	// the context ended later, but claude had already finished on its own
	res, err := finish(exited(t, "exit 3"), context.DeadlineExceeded, &cancelState{}, []byte("out"), []byte("err"))
	if err != nil || res.code != 3 || string(res.stdout) != "out" || string(res.stderr) != "err" {
		t.Errorf("finish = %+v, %v", res, err)
	}
	res, err = finish(nil, context.Canceled, &cancelState{}, []byte("out"), nil)
	if err != nil || res.code != 0 || string(res.stdout) != "out" {
		t.Errorf("finish = %+v, %v", res, err)
	}
}

func TestPipesLeftOpenAfterASuccessfulExitAreNotAnError(t *testing.T) {
	res, err := finish(exec.ErrWaitDelay, nil, &cancelState{}, []byte(`{"loggedIn":true}`), nil)
	if err != nil || string(res.stdout) != `{"loggedIn":true}` {
		t.Errorf("finish = %+v, %v", res, err)
	}
}

func TestACancelIsTypedAndSaysHowClaudeEnded(t *testing.T) {
	cases := []struct {
		name   string
		runErr error
		st     *cancelState
		killed bool
	}{
		{"stopped itself after the interrupt", exited(t, "exit 130"), sentAgo(time.Second), false},
		{"exit 0 after the interrupt", context.Canceled, sentAgo(time.Second), false},
		{"killed once the grace was over", exited(t, "exit 1"), sentAgo(cancelGrace + time.Second), true},
		{"died of a kill", exited(t, "kill -9 $$"), sentAgo(time.Second), true},
	}
	for _, c := range cases {
		_, err := finish(c.runErr, context.Canceled, c.st, nil, nil)

		var ce *CancelError
		if !errors.As(err, &ce) || ce.Killed != c.killed {
			t.Errorf("%s: err = %v, want a CancelError with Killed=%v", c.name, err, c.killed)
			continue
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("%s: err does not wrap the context error", c.name)
		}
	}
}

func TestOtherRunErrorsPassThrough(t *testing.T) {
	boom := fmt.Errorf("fork failed")
	if _, err := finish(boom, nil, &cancelState{}, nil, nil); !errors.Is(err, boom) {
		t.Errorf("err = %v", err)
	}
}
