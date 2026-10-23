package dashboard

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

type confirmKind int

const (
	confirmStop confirmKind = iota
	confirmSignOut
	confirmDelete
)

// confirm is a question answered with y before a session is stopped or an
// account is signed out. any other key cancels it.
type confirm struct {
	open      bool
	kind      confirmKind
	id        string
	label     string // sign out: the account as the question names it
	sessionID string
	state     SessionState
	folder    string
	email     string // delete: shown in the box
	text      string // delete: what has been typed so far
	note      string // delete: why the last answer was refused
}

func (m model) askStop(a Account) (model, tea.Cmd) {
	if m.view == viewSessions {
		for _, s := range a.Sessions {
			if s.ID == m.sessID {
				m.confirm = confirm{open: true, id: a.ID, sessionID: s.ID, state: s.State, folder: s.Folder}
				return m, nil
			}
		}
	}
	return m.say("No session to stop here.", false)
}

func (m model) askSignOut(a Account) model {
	m.confirm = confirm{open: true, kind: confirmSignOut, id: a.ID, label: a.Label}
	return m
}

// confirmKey acts on y and cancels on anything else
func (m model) confirmKey(k string) (model, tea.Cmd) {
	c := m.confirm
	m.confirm = confirm{}
	if k != "y" {
		return m, nil
	}
	act, ctx := m.act, m.ctx
	if c.kind == confirmSignOut {
		return m.working(m.actCmd("Signed out "+c.label+".", func() error { return act.SignOut(ctx, c.id) }))
	}
	return m.working(m.actCmd("Stopped the session.", func() error { return act.StopSession(ctx, c.id, c.sessionID) }))
}

func (m model) confirmLine() rich {
	if m.confirm.kind == confirmDelete {
		return rich{gap(2), bold("Enter"), dim(" deletes "), bold(m.confirm.label), dim(" once its name is typed. Esc cancels.")}.clip(m.w)
	}
	if m.confirm.kind == confirmSignOut {
		return m.signOutLine()
	}
	c := m.confirm
	tail := " y stops it, any other key cancels."
	line := func(what, tail string) rich {
		return rich{gap(2), bold("Stop " + what + "?"), dim(tail)}
	}
	return firstFit(m.w,
		line(fmt.Sprintf("the %s session in %s", c.state, c.folder), tail),
		line(fmt.Sprintf("the %s session", c.state), tail),
		line("this session", tail),
		line("it", " y stops it, other keys cancel."),
	)
}

// signOutLine is the whole question, in shorter wordings as the screen narrows
func (m model) signOutLine() rich {
	label := m.confirm.label
	line := func(mid, tail string) rich {
		return rich{gap(2), bold("y"), dim(" signs out "), bold(label), dim(mid + tail)}
	}
	const kept = " and hides it. Chat history is kept."
	return firstFit(m.w,
		line(kept, " Any other key cancels."),
		line(kept, " Other keys cancel."),
		line(", history is kept.", " Other keys cancel."),
		line(".", " Other keys cancel."),
	)
}
