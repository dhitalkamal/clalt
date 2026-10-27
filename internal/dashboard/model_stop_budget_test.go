package dashboard

import (
	"strings"
	"testing"
)

func TestStopAsksBeforeItStopsAnything(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	m = pressSettle(m, "2")
	asked, _ := Press(m, "x")
	out := Screen(asked)
	if !strings.Contains(out, "Stop the working session in ~/Projects/personal/clalt?") || !strings.Contains(out, "y stops it") {
		t.Errorf("x asks first, naming the session:\n%s", out)
	}
	if got := act.log(); len(got) != 0 {
		t.Fatalf("nothing is stopped yet: %v", got)
	}
	confirmed := pressSettle(asked.(model), "y")
	if got := act.log(); len(got) != 1 || got[0] != "stop a1 s1" {
		t.Errorf("y stops session s1 of account a1: %v", got)
	}
	if strings.Contains(Screen(confirmed), "Stop the working") {
		t.Error("the question goes away once answered")
	}
}

func TestStopTargetsTheSessionUnderTheCursor(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	m = pressSettle(m, "2", "tab", "j")
	pressSettle(m, "x", "y")
	if got := act.log(); len(got) != 1 || got[0] != "stop a1 s2" {
		t.Errorf("the second session is under the cursor: %v", got)
	}
}

func TestAnyOtherKeyCancelsTheStopQuestion(t *testing.T) {
	for _, key := range []string{"esc", "n", "x", "j", "enter", "q"} {
		act := &fakeActions{}
		m, _ := actionModel(act)
		next, _ := Press(m, "2", "x")
		next, cmd := Press(next, key)
		if Quits(cmd) {
			t.Errorf("%s: answering the question never quits", key)
		}
		if strings.Contains(Screen(next), "Stop the working") || len(act.log()) != 0 {
			t.Errorf("%s should cancel without stopping anything (calls %v)", key, act.log())
		}
	}
}

func TestStopHasNothingToDoOutsideARunningSession(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	elsewhere := pressSettle(m, "x")
	if !strings.Contains(Screen(elsewhere), "No session to stop here.") {
		t.Errorf("x outside the sessions view says so:\n%s", Screen(elsewhere))
	}
	none := pressSettle(m, "j", "2", "x")
	if !strings.Contains(Screen(none), "No session to stop here.") || len(act.log()) != 0 {
		t.Errorf("morgan runs nothing:\n%s", Screen(none))
	}
}

func TestBudgetEntryAsksDailyThenWeekly(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	asked, _ := Press(m, "e")
	out := Screen(asked)
	if !strings.Contains(out, "Daily budget for robin in dollars, empty for none: 10.00") {
		t.Errorf("the first question starts from the current cap:\n%s", out)
	}
	next, _ := Press(asked, "backspace", "backspace", "backspace", "backspace", "backspace")
	next = TypeText(next, "12.5")
	next, _ = Press(next, "enter")
	if out := Screen(next); !strings.Contains(out, "Weekly budget for robin in dollars, empty for none: 50.00") {
		t.Fatalf("the second question follows:\n%s", out)
	}
	done := pressSettle(next.(model), "enter")
	if got := act.log(); len(got) != 1 || got[0] != "budget a1 12.50 50.00" {
		t.Errorf("both answers go to SetBudget by id: %v", got)
	}
	if !strings.Contains(Screen(done), "Saved the budget for robin.") {
		t.Errorf("the save is confirmed:\n%s", Screen(done))
	}
}

func TestBudgetEntryTakesNumbersOnlyAndCanBeCancelled(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	next, _ := Press(m, "e", "backspace", "backspace", "backspace", "backspace", "backspace")
	next = TypeText(next, "1x2.5$")
	if out := Screen(next); !strings.Contains(out, "empty for none: 12.5") {
		t.Errorf("only digits and a dot go in:\n%s", out)
	}
	next, _ = Press(next, "backspace", "backspace", "backspace", "backspace", "backspace")
	next = TypeText(next, "1.2.3")
	next, _ = Press(next, "enter")
	out := Screen(next)
	if !strings.Contains(out, "Daily budget") || !strings.Contains(out, "Enter an amount like 10.00.") {
		t.Errorf("a malformed number stays on the question with a note:\n%s", out)
	}
	cancelled, cmd := Press(next, "esc")
	if Quits(cmd) || strings.Contains(Screen(cancelled), "Daily budget") || len(act.log()) != 0 {
		t.Errorf("esc cancels without saving or quitting (calls %v)", act.log())
	}
}

func TestEmptyBudgetAnswersClearTheCaps(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	next, _ := Press(m, "e")
	for i := 0; i < 12; i++ {
		next, _ = Press(next, "backspace")
	}
	next, _ = Press(next, "enter")
	for i := 0; i < 12; i++ {
		next, _ = Press(next, "backspace")
	}
	pressSettle(next.(model), "enter")
	if got := act.log(); len(got) != 1 || got[0] != "budget a1 0.00 0.00" {
		t.Errorf("empty answers mean no cap: %v", got)
	}
}

func TestBudgetEntryUsesTheRealCursor(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	next, _ := Press(m, "e")
	v := next.View()
	if v.Cursor == nil || v.Cursor.Position.Y != 29 {
		t.Fatalf("the cursor sits on the footer line while typing: %+v", v.Cursor)
	}
	line := strings.Split(Screen(next), "\n")[29]
	if want := len(strings.TrimRight(line, " ")); v.Cursor.Position.X != want {
		t.Errorf("the cursor sits after the typed text: column %d, text ends at %d in %q", v.Cursor.Position.X, want, line)
	}
}

func TestBudgetNeedsASignedInAccount(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	out := Screen(pressSettle(m, "j", "j", "j", "j", "e"))
	if !strings.Contains(out, "harriet is signed out. Press s to sign in.") || strings.Contains(out, "Daily budget") {
		t.Errorf("no budget editing while signed out:\n%s", out)
	}
}
