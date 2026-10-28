package dashboard

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// recordLaunch is a Launch that is not a program: it runs inside the test
// binary, the way an adapter runs its own code with the terminal released.
type recordLaunch struct {
	ran      atomic.Int32
	err      error
	returned atomic.Int64 // when Run returned, in unix nanoseconds
}

func (l *recordLaunch) Run() error {
	l.ran.Add(1)
	l.returned.Store(time.Now().UnixNano())
	return l.err
}

func (l *recordLaunch) SetStdin(io.Reader)  {}
func (l *recordLaunch) SetStdout(io.Writer) {}
func (l *recordLaunch) SetStderr(io.Writer) {}

// statusError is how an in process launch says that what it ran ended with a
// status
type statusError struct{ code int }

func (e statusError) Error() string { return fmt.Sprint("ended with status ", e.code) }
func (e statusError) ExitCode() int { return e.code }

func TestALaunchThatIsNotAProgramGoesThroughExec(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	_, cmd := upd(m, launchMsg{launch: &recordLaunch{}})
	if got := fmt.Sprintf("%T", RunCmd(cmd, Patience)); got != "tea.execMsg" {
		t.Errorf("the dashboard hands any launch to tea.Exec so the screen is released, got %s", got)
	}
}

func TestTheProgramRunsALaunchInProcessAndResumesWithAFreshLoad(t *testing.T) {
	src := &fakeSource{snap: fiveSnapshot()}
	l := &recordLaunch{}
	act := &fakeActions{launch: l}
	p, _, done, closeIn := start(t, src, act)
	defer closeIn()

	waitFor(t, "the first load", func() bool { return src.calls.Load() == 1 })
	p.Send(KeyMsg("enter"))
	waitFor(t, "the reload after the launch", func() bool { return src.calls.Load() == 2 })
	if got := l.ran.Load(); got != 1 {
		t.Errorf("the launch ran %d times, want once", got)
	}
	p.Send(KeyMsg("q"))
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("the program ended with %v", err)
		}
	case <-time.After(Patience):
		t.Fatal("the program did not quit after q")
	}
}

// a launch that fails at once, a refusal that was already shown, must not
// hand the terminal back so fast that the renderer is caught out
func TestTheProgramHoldsALaunchThatReturnsAtOnce(t *testing.T) {
	was := minRun
	minRun = 300 * time.Millisecond
	t.Cleanup(func() { minRun = was })
	src := &fakeSource{snap: fiveSnapshot()}
	l := &recordLaunch{err: errors.New("refused")}
	p, _, done, closeIn := start(t, src, &fakeActions{launch: l})
	defer closeIn()

	waitFor(t, "the first load", func() bool { return src.calls.Load() == 1 })
	p.Send(KeyMsg("enter"))
	waitFor(t, "the reload after the launch", func() bool { return src.calls.Load() == 2 })
	// the launch itself came back at once, the program took the terminal back later
	if gap := time.Since(time.Unix(0, l.returned.Load())); gap < minRun {
		t.Errorf("the terminal was back %v after the launch returned, the least is %v", gap, minRun)
	}
	p.Send(KeyMsg("ctrl+c"))
	select {
	case <-done:
	case <-time.After(Patience):
		t.Fatal("the program did not quit after ctrl+c")
	}
}

func TestHeldLaunchKeepsWhatTheLaunchReturned(t *testing.T) {
	inner := &recordLaunch{err: errors.New("refused")}
	start := time.Now()

	err := heldLaunch{Launch: inner, min: 60 * time.Millisecond}.Run()

	if took := time.Since(start); took < 60*time.Millisecond {
		t.Errorf("returned after %v", took)
	}
	if err == nil || err.Error() != "refused" || inner.ran.Load() != 1 {
		t.Errorf("err = %v after %d runs, want the launch's own error from one", err, inner.ran.Load())
	}
}

func TestAnExitStatusOfAnyLaunchIsSaidInAPlainFlash(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	shown, _ := upd(m, execDoneMsg{err: statusError{4}})
	if got := footerOf(shown); got != "  claude exited with status 4" || shown.flash.err {
		t.Errorf("a status is information, not a failure: %q (error %v)", got, shown.flash.err)
	}
	quiet, _ := upd(m, execDoneMsg{err: statusError{130}})
	if quiet.flash.text != "" {
		t.Errorf("status 130 is ctrl+c: %q", quiet.flash.text)
	}
}

func TestALaunchErrorWithoutAStatusIsRed(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	shown, _ := upd(m, execDoneMsg{err: errors.New("run failed: work is signed out")})
	if got := footerOf(shown); got != "  run failed: work is signed out" || !shown.flash.err {
		t.Errorf("%q (error %v)", got, shown.flash.err)
	}
}

func TestCommandOnlyFillsTheStreamsTheCommandLacks(t *testing.T) {
	in, out, errw := strings.NewReader("x"), &bytes.Buffer{}, &bytes.Buffer{}
	bare := exec.Command("clalt-test-program")
	l := Command(bare)
	l.SetStdin(in)
	l.SetStdout(out)
	l.SetStderr(errw)
	if bare.Stdin != in || bare.Stdout != out || bare.Stderr != errw {
		t.Errorf("a command with no streams gets the terminal's: %v %v %v", bare.Stdin, bare.Stdout, bare.Stderr)
	}

	own := &bytes.Buffer{}
	kept := exec.Command("clalt-test-program")
	kept.Stdout = own
	Command(kept).SetStdout(out)
	if kept.Stdout != own {
		t.Error("a stream the command already has is kept")
	}
}

func TestCommandRunsTheProgramAndReportsItsStatus(t *testing.T) {
	if err := Command(helperCmd(0)).Run(); err != nil {
		t.Errorf("a program that succeeds: %v", err)
	}
	err := Command(helperCmd(3)).Run()
	var status interface{ ExitCode() int }
	if !errors.As(err, &status) || status.ExitCode() != 3 {
		t.Errorf("a program that exits with 3 gives a status error, got %v", err)
	}
}

func TestCommandOfNothingIsNothingAndAMissingProgramSaysSo(t *testing.T) {
	if Command(nil) != nil {
		t.Error("no command is no launch, so the dashboard can say that nothing was started")
	}
	l := Command(exec.Command("clalt-test-no-such-program"))
	starter, ok := l.(interface{ StartError() error })
	if !ok || starter.StartError() == nil {
		t.Errorf("a program that is not there is known before it is run: %#v", l)
	}
	if ok := Command(helperCmd(0)).(interface{ StartError() error }).StartError(); ok != nil {
		t.Errorf("a program that is there has no start error: %v", ok)
	}
}
