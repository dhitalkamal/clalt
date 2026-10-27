package dashboard

import (
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

const maxLabelLen = 32

const (
	noteEmpty   = "Enter a name."
	noteControl = "No control characters."
	noteLong    = "Use 32 characters or fewer."
	noteTaken   = "Another account has that name."
)

// checkLabel trims a new label and says what is wrong with it, or returns an
// empty problem. the account being renamed does not count as a duplicate, so
// changing only the case of its own name is allowed.
func checkLabel(label, id string, accounts []Account) (string, string) {
	label = strings.TrimSpace(label)
	switch {
	case label == "":
		return label, noteEmpty
	case strings.ContainsFunc(label, unicode.IsControl):
		return label, noteControl
	case utf8.RuneCountInString(label) > maxLabelLen:
		return label, noteLong
	}
	for _, a := range accounts {
		if a.ID != id && strings.EqualFold(a.Label, label) {
			return label, noteTaken
		}
	}
	return label, ""
}

func (m model) askRename(a Account) model {
	m.entry = entry{open: true, kind: kindRename, id: a.ID, label: a.Label, text: a.Label}
	return m
}

// typeLabel takes printable text, and printable ascii only in ascii mode.
// control characters never reach the footer, and the 32nd character is the last.
func (m model) typeLabel(s string) model {
	for _, r := range s {
		switch {
		case unicode.IsControl(r) || (m.opt.ASCII && r > '~'):
		case utf8.RuneCountInString(m.entry.text) >= maxLabelLen:
			m.entry.note = noteLong
		default:
			m.entry.text += string(r)
			m.entry.note = ""
		}
	}
	return m
}

func (m model) submitRename() (model, tea.Cmd) {
	e := m.entry
	if strings.TrimSpace(e.text) == e.label {
		m.entry = entry{}
		return m.say("Label unchanged.", false)
	}
	label, problem := checkLabel(e.text, e.id, m.snap.Accounts)
	if problem != "" {
		m.entry.note = problem
		return m, nil
	}
	m.entry = entry{}
	act, ctx := m.act, m.ctx
	return m.working(m.actCmd("Renamed "+e.label+" to "+label+".", func() error { return act.Rename(ctx, e.id, label) }))
}
