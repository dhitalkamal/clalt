package dashboard

import (
	"fmt"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

type entryKind int

const (
	kindBudget entryKind = iota
	kindRename
)

// entry is a question in the footer that takes typed text. the budget asks the
// daily cap, then the weekly one. a rename asks for the new label.
type entry struct {
	open      bool
	kind      entryKind
	id        string
	label     string
	step      int // budget: 0 asks for the daily cap, 1 for the weekly one
	daily     float64
	weeklyNow float64 // budget: the current weekly cap, offered as the second answer
	text      string
	note      string // what was wrong with the last answer
}

func (e entry) question() string {
	if e.kind == kindRename {
		return fmt.Sprintf("New label for %s: ", e.label)
	}
	which := "Daily"
	if e.step == 1 {
		which = "Weekly"
	}
	return fmt.Sprintf("%s budget for %s in dollars, empty for none: ", which, e.label)
}

func (m model) entryKey(msg tea.KeyPressMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.entry = entry{}
	case "enter":
		return m.submitEntry()
	case "backspace":
		if _, n := utf8.DecodeLastRuneInString(m.entry.text); n > 0 {
			m.entry.text, m.entry.note = m.entry.text[:len(m.entry.text)-n], ""
		}
	default:
		m = m.typeEntry(msg.Text)
	}
	return m, nil
}

func (m model) typeEntry(s string) model {
	if m.entry.kind == kindRename {
		return m.typeLabel(s)
	}
	return m.typeAmount(s)
}

func (m model) submitEntry() (model, tea.Cmd) {
	if m.entry.kind == kindRename {
		return m.submitRename()
	}
	return m.submitBudget()
}
