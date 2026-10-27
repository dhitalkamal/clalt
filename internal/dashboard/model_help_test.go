package dashboard

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// shortHelp is the help overlay open on the smallest screen, where it scrolls
func shortHelp() model {
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, testOptions(), 60, 16)
	return pressSettle(m, "?")
}

func TestHelpScrollsWithTheArrowKeysOnAShortScreen(t *testing.T) {
	m := shortHelp()
	if out := Screen(m); !strings.Contains(out, "up/down, k/j") || strings.Contains(out, "quit from anywhere") {
		t.Fatalf("help opens at the top:\n%s", out)
	}
	for i := 0; i < 40; i++ {
		m = pressSettle(m, "j")
	}
	if out := Screen(m); !strings.Contains(out, "quit from anywhere") || strings.Contains(out, "up/down, k/j") {
		t.Errorf("j scrolls to the end:\n%s", out)
	}
	for i := 0; i < 40; i++ {
		m = pressSettle(m, "up")
	}
	if out := Screen(m); !strings.Contains(out, "up/down, k/j") || m.helpAt != 0 {
		t.Errorf("up scrolls back to the top:\n%s", out)
	}
}

func TestHelpScrollStopsAtTheEndsInsteadOfPilingUp(t *testing.T) {
	m := shortHelp()
	atTop := pressSettle(m, "k", "up", "k")
	if atTop.helpAt != 0 {
		t.Errorf("scrolling up at the top does nothing: %d", atTop.helpAt)
	}
	for i := 0; i < 50; i++ {
		m = pressSettle(m, "down")
	}
	end := m.helpAt
	if end != m.th.helpLayout(m.w, m.h).maxAt() || end == 0 {
		t.Fatalf("the offset stops at the last position: %d", end)
	}
	if back := pressSettle(m, "k"); back.helpAt != end-1 {
		t.Errorf("one step up after pressing down 50 times moves one line, not 50: %d, end %d", back.helpAt, end)
	}
}

func TestHelpDoesNotScrollWhenItFits(t *testing.T) {
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, testOptions(), 100, 30)
	open := pressSettle(m, "?")
	before := Screen(open)
	scrolled := pressSettle(open, "j", "down", "j")
	if scrolled.helpAt != 0 || Screen(scrolled) != before {
		t.Errorf("a screen that shows every row has nothing to scroll: %d", scrolled.helpAt)
	}
}

func TestOpeningHelpAgainStartsAtTheTop(t *testing.T) {
	m := pressSettle(shortHelp(), "j", "j", "j")
	if m.helpAt == 0 {
		t.Fatal("setup: help did not scroll")
	}
	again := pressSettle(m, "esc", "?")
	if again.helpAt != 0 || !strings.Contains(Screen(again), "up/down, k/j") {
		t.Errorf("each opening starts at the top: %d", again.helpAt)
	}
}

func TestResizingKeepsTheHelpScrollInsideTheList(t *testing.T) {
	m := shortHelp()
	for i := 0; i < 50; i++ {
		m = pressSettle(m, "j")
	}
	next, _ := upd(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if next.helpAt != 0 {
		t.Errorf("a taller screen shows everything, so there is no offset: %d", next.helpAt)
	}
}

func TestHelpIgnoresEveryOtherKey(t *testing.T) {
	act := &fakeActions{}
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, act, testOptions(), 60, 16)
	m = pressSettle(m, "?")
	for _, k := range []string{"enter", "tab", "x", "e", "r", "d", "s", "2", "/", "a", "backspace", "h", "l"} {
		next := pressSettle(m, k)
		if !next.help || next.focus != m.focus || next.view != m.view || next.entry.open || next.confirm.open || next.filtering {
			t.Errorf("%s changed something behind the help overlay", k)
		}
	}
	if len(act.log()) != 0 {
		t.Errorf("no action runs from the help overlay: %v", act.log())
	}
}
