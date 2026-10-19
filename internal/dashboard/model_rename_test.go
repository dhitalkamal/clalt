package dashboard

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// blankRename opens the rename prompt on robin and clears the label it starts with
func blankRename(m model) tea.Model {
	next, _ := Press(m, "r", "backspace", "backspace", "backspace", "backspace", "backspace")
	return next
}

func TestRenameAsksForANewLabelStartingFromTheCurrentOne(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	asked, _ := Press(m, "r")
	if got := footerOf(asked.(model)); got != "  New label for robin: robin" {
		t.Errorf("the prompt names the account and offers its label: %q", got)
	}
	if len(act.log()) != 0 {
		t.Errorf("asking changes nothing: %v", act.log())
	}
}

func TestRenameSavesByIDAndSaysSo(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	next := TypeText(blankRename(m), "work")
	done := pressSettle(next.(model), "enter")
	if got := act.log(); len(got) != 1 || got[0] != "rename a1 work" {
		t.Errorf("the id stays, the label changes: %v", got)
	}
	if got := footerOf(done); got != "  Renamed robin to work." {
		t.Errorf("the save is confirmed: %q", got)
	}
}

func TestRenameTrimsTheLabelBeforeSaving(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	next := TypeText(blankRename(m), "  work ")
	pressSettle(next.(model), "enter")
	if got := act.log(); len(got) != 1 || got[0] != "rename a1 work" {
		t.Errorf("surrounding spaces are not part of the label: %v", got)
	}
}

func TestEscCancelsTheRenameWithoutSavingOrQuitting(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	next := TypeText(blankRename(m), "work")
	cancelled, cmd := Press(next, "esc")
	if Quits(cmd) || strings.Contains(Screen(cancelled), "New label") || len(act.log()) != 0 {
		t.Errorf("esc drops the prompt: calls %v", act.log())
	}
	if got := cancelled.(model).entry; got.open || got.text != "" {
		t.Errorf("nothing of the cancelled prompt is kept: %+v", got)
	}
}

func TestRenameShowsWhatIsWrongInTheFooterAndKeepsAsking(t *testing.T) {
	cases := []struct{ name, typed, note string }{
		{"empty", "", "Enter a name."},
		{"only spaces", "   ", "Enter a name."},
		{"taken", "morgan", "Another account has that name."},
		{"taken in another case", "JORDAN", "Another account has that name."},
	}
	for _, c := range cases {
		act := &fakeActions{}
		m, _ := actionModel(act)
		next := TypeText(blankRename(m), c.typed)
		next, _ = Press(next, "enter")
		foot := footerOf(next.(model))
		if !strings.Contains(foot, "New label for robin: "+c.typed) || !strings.HasSuffix(foot, "  "+c.note) {
			t.Errorf("%s: the footer says what is wrong: %q", c.name, foot)
		}
		if len(act.log()) != 0 || !next.(model).entry.open {
			t.Errorf("%s: nothing is saved and the prompt stays (calls %v)", c.name, act.log())
		}
		next, _ = Press(next, "x")
		if strings.Contains(footerOf(next.(model)), c.note) {
			t.Errorf("%s: typing clears the note", c.name)
		}
	}
}

func TestRenameStopsTypingAtThirtyTwoCharactersAndSaysWhy(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	next := TypeText(blankRename(m), strings.Repeat("a", 31))
	next = TypeText(next, "bcd")
	got := next.(model).entry
	if got.text != strings.Repeat("a", 31)+"b" || got.note != "Use 32 characters or fewer." {
		t.Errorf("the 32nd character fits and the next ones do not: %q, note %q", got.text, got.note)
	}
	next, _ = Press(next, "backspace")
	if got := next.(model).entry; got.note != "" || len(got.text) != 31 {
		t.Errorf("backspace makes room and clears the note: %+v", got)
	}
}

func TestRenameDropsControlCharactersFromAPaste(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	in, _ := upd(blankRename(m).(model), tea.PasteMsg{Content: "new\tname\x1b\r\n"})
	if got := in.entry.text; got != "newname" {
		t.Errorf("a paste cannot put control characters in the footer: %q", got)
	}
	if foot := footerOf(in); strings.ContainsAny(foot, "\x1b\t\r") {
		t.Errorf("the footer stays clean: %q", foot)
	}
}

func TestRenameTakesPrintableAsciiOnlyInAsciiMode(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	in, _ := upd(blankRename(m).(model), tea.PasteMsg{Content: "caf\u00e9 x"})
	if got := in.entry.text; got != "caf x" {
		t.Errorf("ascii mode keeps the footer ascii: %q", got)
	}
	opt := testOptions()
	opt.ASCII = false
	uni := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, opt, 100, 30)
	in, _ = upd(blankRename(uni).(model), tea.PasteMsg{Content: "caf\u00e9"})
	if got := in.entry.text; got != "caf\u00e9" {
		t.Errorf("unicode mode takes letters with accents: %q", got)
	}
}

func TestRenameBackspaceRemovesAWholeCharacter(t *testing.T) {
	opt := testOptions()
	opt.ASCII = false
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, opt, 100, 30)
	in, _ := upd(blankRename(m).(model), tea.PasteMsg{Content: "a\u00e9"})
	next, _ := Press(in, "backspace")
	if got := next.(model).entry.text; got != "a" {
		t.Errorf("backspace never leaves half a character: %q", got)
	}
}

func TestRenameToTheSameLabelSaysSoAndSavesNothing(t *testing.T) {
	for name, typed := range map[string]string{"untouched": "", "with spaces": "  "} {
		act := &fakeActions{}
		m, _ := actionModel(act)
		next, _ := Press(m, "r")
		next = TypeText(next, typed)
		done := pressSettle(next.(model), "enter")
		if got := footerOf(done); got != "  Label unchanged." || len(act.log()) != 0 || done.entry.open {
			t.Errorf("%s: footer %q, calls %v", name, got, act.log())
		}
	}
}

func TestRenameMayChangeOnlyTheCase(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	next := TypeText(blankRename(m), "ROBIN")
	pressSettle(next.(model), "enter")
	if got := act.log(); len(got) != 1 || got[0] != "rename a1 ROBIN" {
		t.Errorf("an account does not clash with its own name: %v", got)
	}
}

func TestRenameOfASignedOutAccountIsFine(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	asked, _ := Press(m, "j", "j", "j", "j", "r")
	if got := footerOf(asked.(model)); got != "  New label for harriet: harriet" {
		t.Errorf("a signed out account can be renamed: %q", got)
	}
}

func TestRenameFromTheRightPaneSaysWhereItWorks(t *testing.T) {
	act := &fakeActions{}
	m, _ := actionModel(act)
	next := pressSettle(m, "tab", "r")
	if got := footerOf(next); got != "  Press tab to pick an account, then r to rename it." || next.entry.open || len(act.log()) != 0 {
		t.Errorf("the right pane does not rename: %q", got)
	}
}

func TestRenameWithNoAccountShowingSaysSo(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	next := pressSettle(m, "/", "z", "z", "z", "enter", "r")
	if got := footerOf(next); got != "  No account selected." || next.entry.open {
		t.Errorf("nothing to rename: %q", got)
	}
}

func TestRenameFailureIsShownAndTheSelectionStaysPut(t *testing.T) {
	act := &fakeActions{err: errors.New("boom")}
	m, _ := actionModel(act)
	next := TypeText(blankRename(m), "work")
	done := pressSettle(next.(model), "enter")
	if got := footerOf(done); got != "  boom" || done.sel != "a1" {
		t.Errorf("an adapter error is shown as it is: %q, selected %q", got, done.sel)
	}
}

func TestRenameKeepsTheAccountSelectedByIDWhenTheLabelChanges(t *testing.T) {
	act := &fakeActions{}
	m, src := actionModel(act)
	next := TypeText(blankRename(m), "work")
	next, cmd := Press(next, "enter")
	snap := fiveSnapshot()
	snap.Accounts[0].Label = "work"
	src.set(snap, nil)
	done := Settle(next, cmd).(model)
	if out := Screen(done); done.sel != "a1" || !strings.Contains(out, "> work") || !strings.Contains(detailTitle(out), "work") {
		t.Errorf("the new label shows and the account stays selected:\n%s", out)
	}
}

func TestRenamePromptUsesTheRealCursorAfterTheText(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	next, _ := Press(m, "r")
	v := next.View()
	if v.Cursor == nil || v.Cursor.Position.Y != 29 {
		t.Fatalf("the cursor sits on the footer line: %+v", v.Cursor)
	}
	if want := len("  New label for robin: robin"); v.Cursor.Position.X != want {
		t.Errorf("the cursor is after the text: column %d, want %d", v.Cursor.Position.X, want)
	}
	over := TypeText(blankRename(m), "morgan")
	over, _ = Press(over, "enter")
	if v := over.View(); v.Cursor == nil || v.Cursor.Position.X != len("  New label for robin: morgan") {
		t.Errorf("the note sits after the cursor, not before it: %+v", v.Cursor)
	}
}
