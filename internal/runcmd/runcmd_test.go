package runcmd

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

// the real runner is tried on sh, which every unix has. windows has no sh, and
// none of the programs this package exists for (kitty, tmux, notify-send).
func needSh(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("these tests run sh")
	}
}

func TestExecReturnsStdout(t *testing.T) {
	needSh(t)

	out, err := Exec{}.Run(context.Background(), "sh", "-c", "printf hello")

	if err != nil || string(out) != "hello" {
		t.Fatalf("got %q, %v", out, err)
	}
}

func TestExecReportsANonZeroExitAsAFailureWithStderr(t *testing.T) {
	needSh(t)

	_, err := Exec{}.Run(context.Background(), "sh", "-c", "echo first >&2; echo second >&2; exit 3")

	var f *Failure
	if !errors.As(err, &f) {
		t.Fatalf("err = %v, want a *Failure", err)
	}
	if f.Code != 3 || !strings.Contains(f.Stderr, "second") {
		t.Errorf("Failure = %+v, want code 3 and the whole stderr", f)
	}
	msg := err.Error()
	if !strings.Contains(msg, "sh") || !strings.Contains(msg, "3") || !strings.Contains(msg, "first") || strings.Contains(msg, "second") {
		t.Errorf("Error() = %q, want the program, the code and the first stderr line only", msg)
	}
}

func TestExecKeepsStdoutOfAFailedProgram(t *testing.T) {
	needSh(t)

	out, err := Exec{}.Run(context.Background(), "sh", "-c", "printf partial; exit 1")

	if err == nil || string(out) != "partial" {
		t.Errorf("got %q, %v", out, err)
	}
}

func TestExecCutsALongStderr(t *testing.T) {
	needSh(t)

	_, err := Exec{}.Run(context.Background(), "sh", "-c", "head -c 100000 /dev/zero | tr '\\0' x >&2; exit 1")

	var f *Failure
	if !errors.As(err, &f) {
		t.Fatalf("err = %v, want a *Failure", err)
	}
	if len(f.Stderr) > 8192 {
		t.Errorf("kept %d bytes of stderr", len(f.Stderr))
	}
	if len(err.Error()) > 300 {
		t.Errorf("Error() is %d bytes long", len(err.Error()))
	}
}

func TestExecSaysWhenTheProgramCannotStart(t *testing.T) {
	_, err := Exec{}.Run(context.Background(), "clalt-no-such-program-xyz")

	var f *Failure
	if err == nil || errors.As(err, &f) {
		t.Fatalf("err = %v, want an error that is not a *Failure", err)
	}
}

func TestExecHasLooksOnPath(t *testing.T) {
	needSh(t)

	if !(Exec{}).Has("sh") {
		t.Error("Has(sh) = false")
	}
	if (Exec{}).Has("clalt-no-such-program-xyz") {
		t.Error("Has(clalt-no-such-program-xyz) = true")
	}
}

func TestExecGivesTheProgramNoInput(t *testing.T) {
	needSh(t)
	done := make(chan struct{})

	// cat only ends when its input does
	go func() {
		Exec{}.Run(context.Background(), "sh", "-c", "cat")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the program waited for input")
	}
}

func TestExecStopsAProgramThatOutlastsTheContext(t *testing.T) {
	needSh(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()

	_, err := Exec{}.Run(ctx, "sh", "-c", "sleep 5")

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want the deadline", err)
	}
	if took := time.Since(start); took > 4*time.Second {
		t.Errorf("took %v", took)
	}
}

func TestExecAppliesItsOwnTimeoutWhenTheContextHasNone(t *testing.T) {
	needSh(t)
	start := time.Now()

	_, err := Exec{Timeout: 100 * time.Millisecond}.Run(context.Background(), "sh", "-c", "sleep 5")

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want the deadline", err)
	}
	if took := time.Since(start); took > 4*time.Second {
		t.Errorf("took %v", took)
	}
}
