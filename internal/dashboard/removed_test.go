package dashboard

import (
	"strings"
	"testing"
)

// removedSnapshot is the five accounts and one removed one
func removedSnapshot() Snapshot {
	s := fiveSnapshot()
	s.Removed = []Removed{{ID: "r1", Label: "old", Email: "old@example.com", Folder: "~/.config/clalt/accounts/r1"}}
	return s
}

// removedModel has the removed row selected
func removedModel(act *fakeActions) model {
	src := &fakeSource{snap: removedSnapshot()}
	m := loadedModel(src, act, testOptions(), 100, 30)
	next, _ := Press(m, "j", "j", "j", "j", "j")
	return next.(model)
}

func TestTheRemovedSectionShowsAtTheBottomOfTheAccountsPane(t *testing.T) {
	m := removedModel(&fakeActions{})
	out := Screen(m)
	for _, want := range []string{"Removed", "old", "5 accounts"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "harriet") > strings.Index(out, "Removed") {
		t.Errorf("the section sits under the accounts:\n%s", out)
	}
}

func TestNoRemovedAccountsNoSection(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	if strings.Contains(Screen(m), "Removed") {
		t.Error("the section is omitted when nothing was removed")
	}
}

func TestTheRightPaneOfARemovedRowIsItsProfile(t *testing.T) {
	out := Screen(removedModel(&fakeActions{}))
	for _, want := range []string{"old@example.com", "~/.config/clalt/accounts/r1", "removed"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Overview") {
		t.Errorf("no overview for a removed account:\n%s", out)
	}
}

func TestDOnARemovedRowDeletesItByID(t *testing.T) {
	act := &fakeActions{}
	m := removedModel(act)
	asked, _ := Press(m, "D")
	if !strings.Contains(Screen(asked), "Delete old") {
		t.Fatalf("the delete box opens:\n%s", Screen(asked))
	}
	typed := TypeText(asked, "old")
	pressSettle(typed.(model), "enter")
	if got := act.log(); len(got) != 1 || got[0] != "delete r1" {
		t.Errorf("calls = %v", got)
	}
}

func TestEnterOnARemovedRowSignsItIn(t *testing.T) {
	act := &fakeActions{}
	pressSettle(removedModel(act), "enter")
	if got := act.log(); len(got) != 1 || got[0] != "signin r1" {
		t.Errorf("calls = %v", got)
	}
}

func TestOtherKeysOnARemovedRowOnlyHint(t *testing.T) {
	for _, k := range []string{"s", "e", "r", "d", "x"} {
		act := &fakeActions{}
		done := pressSettle(removedModel(act), k)
		if len(act.log()) != 0 || done.confirm.open || done.entry.open {
			t.Errorf("%s did something: %v", k, act.log())
		}
		if got := footerOf(done); !strings.Contains(got, "removed: D deletes it for good, enter signs it back in") {
			t.Errorf("%s hint = %q", k, got)
		}
	}
}

func TestTheFooterOfARemovedRowNamesItsTwoKeys(t *testing.T) {
	got := footerOf(removedModel(&fakeActions{}))
	if !strings.Contains(got, "D delete") || !strings.Contains(got, "enter sign in") || strings.Contains(got, "budget") {
		t.Errorf("footer = %q", got)
	}
}

func TestTheCursorCanLeaveTheRemovedRowBackUp(t *testing.T) {
	m := removedModel(&fakeActions{})
	next, _ := Press(m, "j")
	if got := next.(model).sel; got != "r1" {
		t.Errorf("down at the end stays: %q", got)
	}
	next, _ = Press(m, "k")
	if got := next.(model).sel; got != "a5" {
		t.Errorf("up goes to the last account: %q", got)
	}
}

func TestOnlyRemovedAccountsStillListThem(t *testing.T) {
	src := &fakeSource{snap: Snapshot{Removed: removedSnapshot().Removed}}
	m := loadedModel(src, &fakeActions{}, testOptions(), 100, 30)
	out := Screen(m)
	if !strings.Contains(out, "Removed") || strings.Contains(out, "No accounts") {
		t.Errorf("screen:\n%s", out)
	}
}

func TestARemovedIDThatRepeatsAnAccountIsRefused(t *testing.T) {
	s := removedSnapshot()
	s.Removed[0].ID = "a1"
	if err := checkSnapshot(s); err == nil {
		t.Error("ids must be unique across both lists")
	}
}
