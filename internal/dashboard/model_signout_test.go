package dashboard

import (
	"errors"
	"strings"
	"testing"
)

const signOutQuestion = "  y signs out robin and hides it. Chat history is kept. Any other key cancels."

func TestSignOutAsksBeforeItDoesAnything(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	asked, _ := Press(m, "d")
	if got := footerOf(asked.(model)); got != signOutQuestion {
		t.Errorf("d names the account and what happens to it: %q", got)
	}
	if len(act.log()) != 0 {
		t.Errorf("nothing is signed out yet: %v", act.log())
	}
}

func TestYSignsOutByIDAndSaysSo(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	asked, _ := Press(m, "d")
	done := pressSettle(asked.(model), "y")
	if got := act.log(); len(got) != 1 || got[0] != "signout a1" {
		t.Errorf("y signs out account a1, the id and not the label: %v", got)
	}
	if got := footerOf(done); got != "  Signed out robin." || done.confirm.open {
		t.Errorf("the question goes away and the result is said: %q", got)
	}
}

func TestAnyOtherKeyCancelsTheSignOutQuestion(t *testing.T) {
	for _, key := range []string{"esc", "n", "d", "j", "enter", "q", "Y", "backspace", "tab"} {
		act := &fakeActions{}
		m, _ := actionModel(act)
		next, _ := Press(m, "d")
		next, cmd := Press(next, key)
		if Quits(cmd) {
			t.Errorf("%s: answering the question never quits", key)
		}
		if got := footerOf(next.(model)); strings.Contains(got, "y signs out") || len(act.log()) != 0 {
			t.Errorf("%s should cancel without signing out (calls %v, footer %q)", key, act.log(), got)
		}
	}
}

func TestSignOutTargetsTheSelectedAccount(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	pressSettle(m, "j", "j", "d", "y")
	if got := act.log(); len(got) != 1 || got[0] != "signout a3" {
		t.Errorf("the third account is selected: %v", got)
	}
}

func TestSignOutOfASignedOutAccountOnlySaysSo(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	next := pressSettle(m, "j", "j", "j", "j", "d")
	if got := footerOf(next); got != "  harriet is already signed out." || next.confirm.open || len(act.log()) != 0 {
		t.Errorf("no question for an account that is already out: %q", got)
	}
}

func TestSignOutFromTheRightPaneSaysWhereItWorks(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	next := pressSettle(m, "tab", "d")
	if got := footerOf(next); got != "  Press tab to pick an account, then d to sign it out." || next.confirm.open || len(act.log()) != 0 {
		t.Errorf("the right pane does not sign out: %q", got)
	}
}

func TestSignOutWithNoAccountShowingSaysSo(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	next := pressSettle(m, "/", "z", "z", "z", "enter", "d")
	if got := footerOf(next); got != "  No account selected." || next.confirm.open {
		t.Errorf("nothing to sign out: %q", got)
	}
}

func TestSignOutFailureIsShownAsAnError(t *testing.T) {
	act := &fakeActions{err: errors.New("keychain is locked")}
	m, _ := actionModel(act)
	done := pressSettle(m, "d", "y")
	if got := footerOf(done); got != "  keychain is locked" || !done.flash.err {
		t.Errorf("a failure is shown as it is, in the error tone: %q (error %v)", got, done.flash.err)
	}
}

// atWidth is the model on a screen of w columns
func atWidth(m model, w int) model {
	m.w, m.h = w, 30
	return m
}

func TestSignOutQuestionNamesTheLabelAndShrinksToFit(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	asked, _ := Press(m, "d")
	base := asked.(model)
	for w := 40; w <= 140; w++ {
		line := plainText(atWidth(base, w).confirmLine())
		if len(line) > w {
			t.Fatalf("w=%d: the question is %d wide: %q", w, len(line), line)
		}
		if !strings.Contains(line, "y signs out") || !strings.Contains(line, "robin") {
			t.Fatalf("w=%d: the key and the account stay in it: %q", w, line)
		}
	}
	for w, want := range map[int]string{
		100: signOutQuestion,
		78:  signOutQuestion,
		77:  "  y signs out robin and hides it. Chat history is kept. Other keys cancel.",
		74:  "  y signs out robin and hides it. Chat history is kept. Other keys cancel.",
		73:  "  y signs out robin, history is kept. Other keys cancel.",
		56:  "  y signs out robin, history is kept. Other keys cancel.",
		55:  "  y signs out robin. Other keys cancel.",
		44:  "  y signs out robin. Other keys cancel.",
	} {
		if got := plainText(atWidth(base, w).confirmLine()); got != want {
			t.Errorf("w=%d:\n got %q\nwant %q", w, got, want)
		}
	}
}

func TestSignOutQuestionBoldsTheKeyAndTheAccount(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	asked, _ := Press(m, "d")
	var bolds []string
	for _, sp := range asked.(model).confirmLine() {
		if sp.t == toneBold {
			bolds = append(bolds, sp.s)
		}
	}
	if len(bolds) != 2 || bolds[0] != "y" || bolds[1] != "robin" {
		t.Errorf("the key and the account stand out: %q", bolds)
	}
}
