package dashboard

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// logCatcher collects what the dashboard logs
type logCatcher struct {
	mu    sync.Mutex
	lines []string
}

func (l *logCatcher) log(s string) {
	l.mu.Lock()
	l.lines = append(l.lines, s)
	l.mu.Unlock()
}

func (l *logCatcher) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.lines...)
}

func loggedModel(act *fakeActions) (model, *logCatcher) {
	lc := &logCatcher{}
	opt := testOptions()
	opt.Logger = lc.log
	return loadedModel(&fakeSource{snap: fiveSnapshot()}, act, opt, 100, 30), lc
}

func TestALoadFailureIsLoggedWithItsFullText(t *testing.T) {
	m, lc := loggedModel(&fakeActions{})
	upd(m, loadedMsg{err: errors.New("state file is unreadable\nopen /x/state.json: permission denied")})
	got := lc.all()
	if len(got) != 1 || !strings.HasPrefix(got[0], "load failed: ") || !strings.Contains(got[0], "state file is unreadable\nopen /x/state.json: permission denied") {
		t.Errorf("the whole message goes to the log, not the first line the header shows: %q", got)
	}
}

func TestATimeoutAndABadSnapshotAreLogged(t *testing.T) {
	lc := &logCatcher{}
	opt := testOptions()
	opt.LoadTimeout = 20 * time.Millisecond
	opt.Logger = lc.log
	d := deafSource{release: make(chan struct{})}
	defer close(d.release)
	slow := newModel(context.Background(), d, &fakeActions{}, opt)
	l, _ := RunCmd(slow.loadCmd(), 2*time.Second).(loadedMsg)
	upd(slow, l)
	bad := fiveSnapshot()
	bad.Accounts[1].ID = bad.Accounts[0].ID
	upd(slow, loadedMsg{snap: bad})
	got := lc.all()
	if len(got) != 2 || !strings.Contains(got[0], "load timed out after 20ms") || !strings.Contains(got[1], `share the id "a1"`) {
		t.Errorf("both reasons are logged: %q", got)
	}
}

func TestAnActionFailureIsLoggedWithItsFullText(t *testing.T) {
	m, lc := loggedModel(&fakeActions{err: errors.New("keychain is locked\nunlock it first")})
	shown := pressSettle(m, "d", "y")
	if got := footerOf(shown); got != "  keychain is locked" {
		t.Fatalf("setup: the footer has the first line only: %q", got)
	}
	got := lc.all()
	if len(got) != 1 || !strings.HasPrefix(got[0], "action failed: ") || !strings.Contains(got[0], "keychain is locked\nunlock it first") {
		t.Errorf("the log has both lines: %q", got)
	}
}

func TestAStartFailureAndAnExitStatusAreLoggedButAQuitIsNot(t *testing.T) {
	m, lc := loggedModel(&fakeActions{})
	upd(m, execDoneMsg{err: errors.New("fork/exec /x/claude: permission denied")})
	upd(m, execDoneMsg{err: exitError(t, 3)})
	upd(m, execDoneMsg{err: exitError(t, 130)})
	upd(m, launchMsg{launch: Command(helperCmd(0)), err: errors.New("no such profile")})
	got := lc.all()
	if len(got) != 3 || !strings.Contains(got[0], "permission denied") || !strings.Contains(got[1], "status 3") || !strings.Contains(got[2], "no such profile") {
		t.Errorf("failures are logged, ctrl+c is not: %q", got)
	}
}

func TestAPanicIsLogged(t *testing.T) {
	act := &panicActions{value: "bad state"}
	m, lc := loggedModel(&act.fakeActions)
	m.act = act
	pressSettle(m, "r", "x", "enter")
	if got := lc.all(); len(got) != 1 || !strings.Contains(got[0], "action panicked: bad state") {
		t.Errorf("a panic is a failure like any other: %q", got)
	}
}

func TestNoticesAndSuccessesAreNotLogged(t *testing.T) {
	m, lc := loggedModel(&fakeActions{err: Notice("Demo: would sign out robin (history kept).")})
	pressSettle(m, "d", "y")
	ok, lc2 := loggedModel(&fakeActions{})
	pressSettle(ok, "d", "y")
	upd(ok, loadedMsg{snap: fiveSnapshot()})
	if len(lc.all()) != 0 || len(lc2.all()) != 0 {
		t.Errorf("a notice is a message and a success is nothing to report: %q %q", lc.all(), lc2.all())
	}
}

func TestTheLogNeverCarriesSnapshotText(t *testing.T) {
	snap := fiveSnapshot()
	snap.Accounts[0].Sessions[0].Folder = "~/SECRET-transcript-words"
	snap.Accounts[0].Problem = "SECRET-problem-text"
	lc := &logCatcher{}
	opt := testOptions()
	opt.Logger = lc.log
	m := loadedModel(&fakeSource{snap: snap}, &fakeActions{err: errors.New("stop failed")}, opt, 100, 30)
	pressSettle(m, "2", "x", "y")
	upd(m, loadedMsg{err: errors.New("down")})
	for _, line := range lc.all() {
		if strings.Contains(line, "SECRET") {
			t.Errorf("only error text is logged: %q", line)
		}
	}
	if len(lc.all()) != 2 {
		t.Errorf("both failures are logged: %q", lc.all())
	}
}

func TestWithoutALoggerNothingBreaks(t *testing.T) {
	m, _ := actionModel(&fakeActions{err: errors.New("boom")})
	if m.opt.Logger != nil {
		t.Fatal("setup: the default has no logger")
	}
	pressSettle(m, "d", "y")
	upd(m, loadedMsg{err: errors.New("down")})
}
