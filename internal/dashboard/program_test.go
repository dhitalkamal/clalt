package dashboard

import (
	"io"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// these tests run a real program on a pipe. the commands they run for claude
// take a moment (see helperCmd): bubbletea v2.1.0 can leave its renderer
// stopped when tea.ExecProcess returns at once, which showed as a hang with
// go test -cpu 1. run them with -cpu 1,4 after touching the exec path.

// syncBuffer collects what the program draws
type syncBuffer struct {
	mu sync.Mutex
	sb strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sb.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sb.String()
}

// start runs a real program on a pipe with no terminal and returns what a test
// needs to drive it
func start(t *testing.T, src *fakeSource, act *fakeActions) (p *tea.Program, drawn *syncBuffer, done <-chan error, closeIn func()) {
	t.Helper()
	in, out := io.Pipe()
	drawn = &syncBuffer{}
	prog := tea.NewProgram(newModel(t.Context(), src, act, testOptions()),
		tea.WithInput(in), tea.WithOutput(drawn), tea.WithWindowSize(100, 30), tea.WithoutSignals())
	errs := make(chan error, 1)
	go func() {
		_, err := prog.Run()
		errs <- err
	}()
	return prog, drawn, errs, func() { out.Close() }
}

// wait fails the test, instead of hanging it, when ch is not closed in time
func wait(t *testing.T, what string, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(Patience):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(Patience)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestTheProgramRunsTheCommandAndResumesWithAFreshLoad(t *testing.T) {
	src := &fakeSource{snap: fiveSnapshot()}
	cmd := helperCmd(0)
	act := &fakeActions{cmd: cmd}
	p, _, done, closeIn := start(t, src, act)
	defer closeIn()

	waitFor(t, "the first load", func() bool { return src.calls.Load() == 1 })
	p.Send(KeyMsg("j"))
	p.Send(KeyMsg("enter"))
	waitFor(t, "the reload after the command", func() bool { return src.calls.Load() == 2 })
	if cmd.ProcessState == nil || !cmd.ProcessState.Success() {
		t.Errorf("the command from Run ran to the end: %v", cmd.ProcessState)
	}
	if got := act.log(); len(got) != 1 || got[0] != "run a2" {
		t.Errorf("the actions saw %v, want one run of account a2", got)
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

func TestTheProgramShowsACommandThatCouldNotStart(t *testing.T) {
	src := &fakeSource{snap: fiveSnapshot()}
	missing := exec.Command("claude-for-the-dashboard-test")
	p, drawn, done, closeIn := start(t, src, &fakeActions{cmd: missing})
	defer closeIn()

	waitFor(t, "the first load", func() bool { return src.calls.Load() == 1 })
	p.Send(KeyMsg("enter"))
	waitFor(t, "the error on screen", func() bool { return strings.Contains(drawn.String(), "claude-for-the-dashboard-test") })
	p.Send(KeyMsg("ctrl+c"))
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("ctrl+c ends the program cleanly, got %v", err)
		}
	case <-time.After(Patience):
		t.Fatal("the program did not quit after ctrl+c")
	}
}

func TestTheProgramSaysWhenTheCommandEndedWithAStatus(t *testing.T) {
	src := &fakeSource{snap: fiveSnapshot()}
	p, drawn, done, closeIn := start(t, src, &fakeActions{cmd: helperCmd(3)})
	defer closeIn()

	waitFor(t, "the first load", func() bool { return src.calls.Load() == 1 })
	p.Send(KeyMsg("enter"))
	waitFor(t, "the status on screen", func() bool { return strings.Contains(drawn.String(), "claude exited with status 3") })
	p.Send(KeyMsg("ctrl+c"))
	select {
	case <-done:
	case <-time.After(Patience):
		t.Fatal("the program did not quit after ctrl+c")
	}
}
