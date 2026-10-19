package dashboard

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestFlashTimerOnlyClearsItsOwnFlash(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	first, _ := m.say("first", false)
	idFirst := first.flash.id
	second, _ := first.say("second", false)
	stale, _ := upd(second, flashExpiredMsg{id: idFirst})
	if stale.flash.text != "second" {
		t.Error("an old timer must not clear a newer flash")
	}
	done, _ := upd(second, flashExpiredMsg{id: second.flash.id})
	if done.flash.text != "" || !strings.Contains(Screen(done), "q quit") {
		t.Error("the flash clears and the key hints come back")
	}
}

func TestFlashArmsATimerOfFlashFor(t *testing.T) {
	opt := testOptions()
	opt.FlashFor = 5 * time.Millisecond
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, opt, 100, 30)
	m, cmd := m.say("hello", false)
	msg := RunCmd(cmd, time.Second)
	expired, ok := msg.(flashExpiredMsg)
	if !ok || expired.id != m.flash.id {
		t.Errorf("the timer reports the flash it belongs to, got %#v", msg)
	}
}

func TestFilterMatchesLabelsNotIDs(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	next, _ := Press(m, "/")
	next = TypeText(next, "a1")
	if out := Screen(next); !strings.Contains(out, "no match") {
		t.Errorf("a1 is an id, the list shows labels:\n%s", out)
	}
	next, _ = Press(next, "esc")
	next, _ = Press(next, "/")
	next = TypeText(next, "AUR")
	if out := Screen(next); !strings.Contains(out, "> jordan") || strings.Contains(out, " robin ") {
		t.Errorf("the filter ignores case:\n%s", out)
	}
}

func TestPasteGoesIntoTheFilterAndTheBudgetEntryOnly(t *testing.T) {
	m, _ := actionModel(&fakeActions{})
	plain, _ := upd(m, tea.PasteMsg{Content: "zzz"})
	if plain.filter != "" {
		t.Error("a paste outside typing is ignored")
	}
	typing, _ := Press(m, "/")
	pasted, _ := upd(typing.(model), tea.PasteMsg{Content: "sh\n"})
	if pasted.filter != "sh" {
		t.Errorf("a paste into the filter keeps printable text, got %q", pasted.filter)
	}
	asked, _ := Press(m, "e")
	cleared, _ := Press(asked, "backspace", "backspace", "backspace", "backspace", "backspace")
	in, _ := upd(cleared.(model), tea.PasteMsg{Content: "7a5"})
	if !strings.Contains(Screen(in), "empty for none: 75") {
		t.Errorf("a paste into the budget entry keeps digits:\n%s", Screen(in))
	}
}

func TestEveryModalStateQuitsOnCtrlCAndNeverOnEscOrIgnoredKeys(t *testing.T) {
	act := &fakeActions{}
	base, _ := actionModel(act)
	filterOpen, _ := Press(base, "/")
	filterTyped := TypeText(filterOpen, "ka")
	filterKept, _ := Press(filterTyped, "enter")
	helpOpen, _ := Press(base, "?")
	budget, _ := Press(base, "e")
	stopAsk, _ := Press(base, "2", "x")
	drilled, _ := Press(base, "4", "tab", "l", "enter")
	renaming, _ := Press(base, "r")
	signOut, _ := Press(base, "d")
	states := map[string]tea.Model{
		"normal": base, "filter empty": filterOpen, "filter typed": filterTyped,
		"filter kept": filterKept, "help": helpOpen, "budget": budget, "stop question": stopAsk,
		"drill-in": drilled, "rename": renaming, "sign out question": signOut,
	}
	for name, st := range states {
		if _, cmd := Press(st, "ctrl+c"); !Quits(cmd) {
			t.Errorf("%s: ctrl+c should quit", name)
		}
		if _, cmd := Press(st, "esc"); Quits(cmd) {
			t.Errorf("%s: esc must never quit", name)
		}
	}
	for name, st := range map[string]tea.Model{"filter typing": filterOpen, "budget": budget, "rename": renaming} {
		if _, cmd := Press(st, "q"); Quits(cmd) {
			t.Errorf("%s: q is text or ignored while typing", name)
		}
	}
	for name, st := range map[string]tea.Model{"normal": base, "help": helpOpen, "filter kept": filterKept, "drill-in": drilled} {
		if _, cmd := Press(st, "q"); !Quits(cmd) {
			t.Errorf("%s: q should quit when nothing is being typed", name)
		}
	}
}
