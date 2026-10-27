package dashboard

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func actionModel(act *fakeActions) (model, *fakeSource) {
	src := &fakeSource{snap: fiveSnapshot()}
	return loadedModel(src, act, testOptions(), 100, 30), src
}

// pressSettle presses keys and then lets every command finish
func pressSettle(m model, keys ...string) model {
	next, cmd := Press(m, keys...)
	return Settle(next, cmd).(model)
}

func TestEnterRunsTheSelectedAccountByID(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	pressSettle(m, "enter")
	if got := act.log(); len(got) != 1 || got[0] != "run a1" {
		t.Errorf("enter should run account a1 (the id, not the label robin): %v", got)
	}
}

func TestSignInUsesTheSelectedAccountsID(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	pressSettle(m, "j", "j", "s")
	if got := act.log(); len(got) != 1 || got[0] != "signin a3" {
		t.Errorf("s should sign in account a3: %v", got)
	}
}

func TestARunCommandSuspendsTheProgramThroughExec(t *testing.T) {
	act := &fakeActions{cmd: helperCmd(0)}
	m, _ := actionModel(act)
	_, cmd := Press(m, "enter")
	launch, ok := RunCmd(cmd, Patience).(launchMsg)
	if !ok || launch.launch == nil || launch.err != nil {
		t.Fatalf("enter asks the actions for a command off the update loop, got %#v", launch)
	}
	_, exec := upd(m, launch)
	if got := fmt.Sprintf("%T", RunCmd(exec, Patience)); got != "tea.execMsg" {
		t.Errorf("the dashboard hands the command to tea.Exec so the screen is released, got %s", got)
	}
}

// bubbletea v2.1.0 can leave its renderer stopped when tea.ExecProcess returns
// at once, so a command that cannot start never reaches it
func TestACommandThatCannotStartIsReportedWithoutLeavingTheScreen(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	missing := exec.Command("clalt-test-no-such-program")
	shown, cmd := upd(m, launchMsg{launch: Command(missing)})
	if got := fmt.Sprintf("%T", RunCmd(cmd, 100*time.Millisecond)); got == "tea.execMsg" {
		t.Error("the screen must not be released for a command that cannot run")
	}
	if !shown.flash.err || !strings.Contains(shown.flash.text, "clalt-test-no-such-program") {
		t.Errorf("the reason is shown in red: %+v", shown.flash)
	}
}

func TestSignInAlsoGoesThroughExec(t *testing.T) {
	act := &fakeActions{cmd: helperCmd(0)}
	m, _ := actionModel(act)
	_, cmd := Press(m, "s")
	launch, ok := RunCmd(cmd, Patience).(launchMsg)
	if !ok || launch.launch == nil {
		t.Fatalf("s asks for an interactive command, got %#v", launch)
	}
}

func TestFinishingAnExecReloadsTheData(t *testing.T) {
	act := &fakeActions{}
	m, src := actionModel(act)
	before := src.calls.Load()
	m, cmd := upd(m, execDoneMsg{})
	Settle(m, cmd)
	if src.calls.Load() != before+1 {
		t.Errorf("claude may have changed logins or usage, so the dashboard reloads (%d to %d)", before, src.calls.Load())
	}
}

// exitError is the error a command that ended with status code gives
func exitError(t *testing.T, code int) error {
	t.Helper()
	err := helperCmd(code).Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("setup: a command that exits with %d gives an ExitError, got %v", code, err)
	}
	return err
}

func TestAnExitStatusIsSaidInAPlainFlash(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	shown, _ := upd(m, execDoneMsg{err: exitError(t, 3)})
	if got := footerOf(shown); got != "  claude exited with status 3" || shown.flash.err {
		t.Errorf("a status is information, not a failure: %q (error %v)", got, shown.flash.err)
	}
}

func TestQuittingClaudeWithCtrlCOrASignalIsQuiet(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	quiet, _ := upd(m, execDoneMsg{err: exitError(t, 130)})
	if quiet.flash.text != "" {
		t.Errorf("status 130 is ctrl+c, a normal way to leave claude: %q", quiet.flash.text)
	}
	if runtime.GOOS == "windows" {
		t.Skip("windows has no signals: a killed process reports status 1")
	}
	killed := helperCmd(0)
	if err := killed.Start(); err != nil {
		t.Fatal(err)
	}
	_ = killed.Process.Kill()
	signalled := killed.Wait()
	if code := killed.ProcessState.ExitCode(); code != -1 {
		t.Fatalf("setup: a killed process has no status, got %d", code)
	}
	quiet, _ = upd(m, execDoneMsg{err: signalled})
	if quiet.flash.text != "" {
		t.Errorf("a signal exit has no status to report: %q", quiet.flash.text)
	}
}

func TestACommandThatCouldNotStartStaysRed(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	shown, _ := upd(m, execDoneMsg{err: errors.New("claude not found in PATH")})
	if got := footerOf(shown); got != "  claude not found in PATH" || !shown.flash.err {
		t.Errorf("a command that could not start says why, in red: %q (error %v)", got, shown.flash.err)
	}
}

func TestNoCommandAndNoErrorIsSaidNotIgnored(t *testing.T) {
	for _, key := range []string{"enter", "s"} {
		m, _ := actionModel(&fakeActions{})
		shown := pressSettle(m, key)
		if got := footerOf(shown); got != "  Nothing was started." || shown.flash.err {
			t.Errorf("%s: an adapter with nothing to run still answers: %q", key, got)
		}
	}
}

func TestANoticeIsAPlainFlashAndAFailureIsRed(t *testing.T) {
	act := &fakeActions{err: Notice("Demo: would start claude as robin.")}
	m, _ := actionModel(act)
	shown := pressSettle(m, "enter")
	if !strings.Contains(Screen(shown), "Demo: would start claude as robin.") {
		t.Fatalf("the notice is the flash:\n%s", Screen(shown))
	}
	if shown.flash.err {
		t.Error("a notice is not a failure")
	}

	failing := &fakeActions{err: errors.New("profile folder is missing")}
	colorOpt := testOptions()
	colorOpt.NoColor = false
	fm := loadedModel(&fakeSource{snap: fiveSnapshot()}, failing, colorOpt, 100, 30)
	failed := pressSettle(fm, "enter")
	if !strings.Contains(Screen(failed), "profile folder is missing") || !failed.flash.err {
		t.Fatalf("a failure shows inline:\n%s", Screen(failed))
	}
	if !strings.Contains(Raw(failed), "\x1b[31mprofile folder is missing") {
		t.Error("an error flash is red, and says so in words as well")
	}
}

// only a bare Notice is a notice. an error that merely contains one is a
// failure, and its own text is what the flash says
func TestAJoinedOrWrappedNoticeDoesNotHideARealError(t *testing.T) {
	cases := map[string]struct {
		err  error
		want string
	}{
		"joined, notice first": {errors.Join(Notice("Demo: would stop that session."), errors.New("disk full")), "  disk full"},
		"joined, notice last":  {errors.Join(errors.New("disk full"), Notice("Demo: would stop that session.")), "  disk full"},
		"wrapped":              {fmt.Errorf("stop failed: %w", Notice("Demo: would stop that session.")), "  stop failed: Demo: would stop that session."},
		"joined notices only":  {errors.Join(Notice("one"), Notice("two")), "  one"},
	}
	for name, c := range cases {
		m, _ := actionModel(&fakeActions{err: c.err})
		shown := pressSettle(m, "2", "x", "y")
		if got := footerOf(shown); got != c.want || !shown.flash.err {
			t.Errorf("%s: %q (error %v), want %q in red", name, got, shown.flash.err, c.want)
		}
	}
	m, _ := actionModel(&fakeActions{err: Notice("Demo: would stop that session.")})
	if shown := pressSettle(m, "2", "x", "y"); footerOf(shown) != "  Demo: would stop that session." || shown.flash.err {
		t.Errorf("a bare notice is still a plain message: %q", footerOf(shown))
	}
}

func TestAnErrorFlashOutlivesANotice(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	_, noticeCmd := m.say("hello", false)
	_, errCmd := m.say("broken", true)
	if noticeCmd == nil || errCmd == nil {
		t.Fatal("every flash arms a timer")
	}
	opt := m.opt
	if opt.FlashFor <= 0 {
		t.Fatal("setup: flash duration")
	}
	if m.flashTTL(true) <= m.flashTTL(false) {
		t.Errorf("errors stay up longer: %v vs %v", m.flashTTL(true), m.flashTTL(false))
	}
}

func TestSignedOutAccountsCannotRunButCanSignIn(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	m = pressSettle(m, "j", "j", "j", "j")
	signedOut := pressSettle(m, "enter")
	if got := act.log(); len(got) != 0 {
		t.Errorf("a signed out account is not started: %v", got)
	}
	if !strings.Contains(Screen(signedOut), "harriet is signed out. Press s to sign in.") {
		t.Errorf("enter explains what to do:\n%s", Screen(signedOut))
	}
	pressSettle(m, "s")
	if got := act.log(); len(got) != 1 || got[0] != "signin a5" {
		t.Errorf("s signs in account a5: %v", got)
	}
}

func TestAnActionErrorShowsInlineAndSuccessReloads(t *testing.T) {
	act := &fakeActions{err: errors.New("no such process")}
	m, src := actionModel(act)
	before := src.calls.Load()
	failed := pressSettle(m, "2", "x", "y")
	if !strings.Contains(Screen(failed), "no such process") || src.calls.Load() != before {
		t.Errorf("a failure is shown and changes nothing (calls %d to %d):\n%s", before, src.calls.Load(), Screen(failed))
	}
	act.err = nil
	ok := pressSettle(failed, "2", "x", "y")
	if !strings.Contains(Screen(ok), "Stopped the session.") || src.calls.Load() != before+1 {
		t.Errorf("success is confirmed and the data reloaded (calls %d to %d):\n%s", before, src.calls.Load(), Screen(ok))
	}
}
