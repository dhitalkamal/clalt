package dashboard

import (
	"errors"
	"strings"
	"testing"
)

func TestDOpensTheDeleteBoxAndDeletesNothingYet(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	asked, _ := Press(m, "D")
	out := Screen(asked.(model))
	for _, want := range []string{
		"Delete robin", "robin@example.com", "~/.config/clalt/profiles/robin",
		"this deletes the account; your conversation history stays",
		"Type robin to confirm",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the box says %q:\n%s", want, out)
		}
	}
	if len(act.log()) != 0 {
		t.Errorf("opening the box deletes nothing: %v", act.log())
	}
}

func TestDeleteWithTheWrongLabelKeepsTheBoxOpenWithAHint(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	asked, _ := Press(m, "D")
	typed := TypeText(asked, "kama")
	next, _ := Press(typed, "enter")
	out := Screen(next)
	if len(act.log()) != 0 || !next.(model).confirm.open {
		t.Errorf("a wrong label keeps the box and calls nothing: %v", act.log())
	}
	if !strings.Contains(out, "Type robin exactly") {
		t.Errorf("the box hints at what to type:\n%s", out)
	}
}

func TestDeleteWithTheRightLabelCallsDeleteOnceByID(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	asked, _ := Press(m, "D")
	typed := TypeText(asked, "robin")
	done := pressSettle(typed.(model), "enter")
	if got := act.log(); len(got) != 1 || got[0] != "delete a1" {
		t.Errorf("the id is deleted once: %v", got)
	}
	if done.confirm.open || done.busy {
		t.Errorf("the box closes and the dashboard is free again")
	}
	if got := footerOf(done); got != "  Deleted robin." {
		t.Errorf("the result is said: %q", got)
	}
}

func TestDeleteLabelIsMatchedExactly(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	asked, _ := Press(m, "D")
	typed := TypeText(asked, "Robin")
	pressSettle(typed.(model), "enter")
	if len(act.log()) != 0 {
		t.Errorf("a different case is a different label: %v", act.log())
	}
}

func TestEscCancelsTheDeleteBoxWithoutQuitting(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	asked, _ := Press(m, "D")
	typed := TypeText(asked, "robin")
	next, cmd := Press(typed, "esc")
	if Quits(cmd) || next.(model).confirm.open || len(act.log()) != 0 {
		t.Errorf("esc drops the box and nothing is deleted: %v", act.log())
	}
	if strings.Contains(Screen(next), "Delete robin") {
		t.Errorf("the box is gone")
	}
}

func TestDeleteBackspaceEditsTheTypedLabel(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	asked, _ := Press(m, "D")
	typed := TypeText(asked, "robinx")
	fixed, _ := Press(typed, "backspace")
	pressSettle(fixed.(model), "enter")
	if len(act.log()) != 1 {
		t.Errorf("backspace removes the last letter: %v", act.log())
	}
}

func TestDeleteRefusalIsShownOnTheFlashLine(t *testing.T) {
	act := &fakeActions{err: errors.New("robin has a running session (pid 42). nothing was changed")}
	m, _ := actionModel(act)
	asked, _ := Press(m, "D")
	typed := TypeText(asked, "robin")
	done := pressSettle(typed.(model), "enter")
	if got := footerOf(done); !strings.Contains(got, "running session") || !done.flash.err {
		t.Errorf("a refusal reads as an error on the footer: %q", got)
	}
	if done.confirm.open {
		t.Errorf("the box is closed after the refusal")
	}
}

func TestDeleteWorksOnASignedOutAccount(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	asked, _ := Press(m, "j", "j", "j", "j", "D")
	if !asked.(model).confirm.open {
		t.Fatalf("a signed out account can be deleted too")
	}
}

func TestDeleteFromTheRightPaneSaysWhereItWorks(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	next := pressSettle(m, "tab", "D")
	if got := footerOf(next); got != "  Press tab to pick an account, then D to delete it." || next.confirm.open {
		t.Errorf("the right pane does not delete: %q", got)
	}
}

func TestLowercaseDStillSignsOut(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	asked, _ := Press(m, "d")
	if got := footerOf(asked.(model)); got != signOutQuestion {
		t.Errorf("d is sign out: %q", got)
	}
}
