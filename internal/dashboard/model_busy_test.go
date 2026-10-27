package dashboard

import (
	"errors"
	"testing"
)

func TestADoubleEnterRunsOnce(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	first, cmd := Press(m, "enter")
	second, again := Press(first, "enter")
	if again != nil {
		t.Error("the second enter starts nothing while the first is in flight")
	}
	Settle(second, cmd)
	if got := act.log(); len(got) != 1 || got[0] != "run a1" {
		t.Errorf("one run, not two: %v", got)
	}
}

func TestTheFooterSaysWorkingWhileAnActionIsInFlight(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	next, _ := Press(m, "enter")
	if got := footerOf(next.(model)); got != "  working..." {
		t.Errorf("the footer while waiting: %q", got)
	}
}

func TestEveryActionKeyIsIgnoredWhileBusy(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	busy, _ := Press(m, "enter")
	for _, key := range []string{"enter", "s", "e", "r", "d", "x"} {
		next, cmd := Press(busy, key)
		got := next.(model)
		if cmd != nil || got.entry.open || got.confirm.open || footerOf(got) != "  working..." {
			t.Errorf("%s while busy: cmd %v, prompt %v, confirm %v, footer %q", key, cmd != nil, got.entry.open, got.confirm.open, footerOf(got))
		}
	}
	if len(act.log()) != 0 {
		t.Errorf("nothing ran beyond the first action's own call: %v", act.log())
	}
}

func TestNavigationAndQuittingStillWorkWhileBusy(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	busy, _ := Press(m, "enter")
	moved, _ := Press(busy, "j", "tab", "2", "?")
	got := moved.(model)
	if got.sel != "a2" || got.focus != paneDetail || got.view != viewSessions || !got.help {
		t.Errorf("moving around is not an action: sel %s focus %v view %v help %v", got.sel, got.focus, got.view, got.help)
	}
	if _, cmd := Press(busy, "q"); !Quits(cmd) {
		t.Error("q quits while an action runs")
	}
	if _, cmd := Press(busy, "ctrl+c"); !Quits(cmd) {
		t.Error("ctrl+c quits while an action runs")
	}
}

func TestOpeningAProjectIsNotAnActionSoItWorksWhileBusy(t *testing.T) {
	m := projectsModel(&fakeActions{})
	busy, _ := Press(m, "tab", "enter", "tab")
	opened, _ := Press(busy, "enter")
	if !opened.(model).drill.open() {
		t.Error("the drill-in is only a view, nothing is in flight for it")
	}
}

func TestBusyEndsWhenTheActionFinishesOrFails(t *testing.T) {
	cases := map[string]func(*fakeActions){
		"no command":    func(a *fakeActions) {},
		"start error":   func(a *fakeActions) { a.err = errors.New("no profile") },
		"cannot launch": func(a *fakeActions) { a.cmd = helperCmd(0); a.cmd.Err = errors.New("not found") },
	}
	for name, setup := range cases {
		act := &fakeActions{}
		setup(act)
		m, _ := actionModel(act)
		next, cmd := Press(m, "enter")
		done := Settle(next, cmd).(model)
		if done.busy || footerOf(done) == "  working..." {
			t.Errorf("%s: still busy after the answer: %q", name, footerOf(done))
		}
		if again := pressSettle(done, "enter"); len(act.log()) != 2 || again.busy && again.flash.text == "" {
			t.Errorf("%s: a new action may start once the first is over: %v", name, act.log())
		}
	}
}

func TestBusyEndsAfterAnExecAndAfterASavedAction(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	busy, _ := Press(m, "enter")
	exited, _ := upd(busy.(model), execDoneMsg{err: exitError(t, 2)})
	if exited.busy {
		t.Error("the command is over, so the dashboard is free again")
	}
	renamed, cmd := Press(m, "r", "x", "enter")
	if !renamed.(model).busy {
		t.Fatal("a rename is in flight until it answers")
	}
	if done := Settle(renamed, cmd).(model); done.busy || footerOf(done) != "  Renamed robin to robinx." {
		t.Errorf("the answer ends it and says so: busy %v, %q", done.busy, footerOf(done))
	}
}

func TestAnEnterRightAfterASubmitIsIgnored(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	renamed, cmd := Press(m, "r", "x", "enter")
	again, runs := Press(renamed, "enter")
	if runs != nil {
		t.Error("enter after submitting must not run claude while the rename is in flight")
	}
	Settle(again, cmd)
	if got := act.log(); len(got) != 1 || got[0] != "rename a1 robinx" {
		t.Errorf("only the rename ran: %v", got)
	}
}
