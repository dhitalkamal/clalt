package dashboard

import (
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// askDelete opens the delete box for the account
func (m model) askDelete(a Account) model {
	m.confirm = confirm{
		open: true, kind: confirmDelete, id: a.ID, label: a.Label,
		email: a.Email, folder: a.Profile.Folder,
	}
	return m
}

// deleteKey types the label into the box. enter acts only when the label
// matches exactly, esc cancels, ctrl+c is handled before this.
func (m model) deleteKey(msg tea.KeyPressMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.confirm = confirm{}
	case "enter":
		return m.submitDelete()
	case "backspace":
		if _, n := utf8.DecodeLastRuneInString(m.confirm.text); n > 0 {
			m.confirm.text, m.confirm.note = m.confirm.text[:len(m.confirm.text)-n], ""
		}
	default:
		m = m.typeDelete(msg.Text)
	}
	return m, nil
}

// typeDelete takes printable text only, so a stray control key is not a letter
func (m model) typeDelete(s string) model {
	for _, r := range s {
		if r >= ' ' && r != 0x7f {
			m.confirm.text += string(r)
			m.confirm.note = ""
		}
	}
	return m
}

func (m model) submitDelete() (model, tea.Cmd) {
	c := m.confirm
	if c.text != c.label {
		m.confirm.note = "That is not " + c.label + ". Type " + c.label + " exactly, or press esc."
		return m, nil
	}
	m.confirm = confirm{}
	act, ctx := m.act, m.ctx
	return m.working(m.actCmd("Deleted "+c.label+".", func() error { return act.Delete(ctx, c.id) }))
}

// deleteBox is the question as a box over the dashboard
func (th theme) deleteBox(c confirm, w, h int) []string {
	bw := min(76, w-2)
	inner := bw - 4
	email := c.email
	if email == "" {
		email = "no email recorded"
	}
	folder := c.folder
	if folder == "" {
		folder = "no folder"
	}
	var rows []rich
	rows = append(rows, hang(rich{bold("Account")}, 9, rich{txt(c.label + " (" + email + ")")}, inner)...)
	rows = append(rows, hang(rich{bold("Folder")}, 9, rich{txt(folder)}, inner)...)
	rows = append(rows, nil, rich{dim("this deletes the account; your conversation history stays")}, nil)
	rows = append(rows, rich{txt("Type " + c.label + " to confirm: "), bold(c.text)})
	if c.note != "" {
		rows = append(rows, rich{bad(c.note)})
	} else {
		rows = append(rows, nil)
	}
	return th.box(boxSpec{
		title: "Delete " + c.label, rows: rows, w: bw, h: len(rows) + 2,
		padL: 1, padR: 1, focused: true, hint: "enter deletes, esc cancels",
	})
}
