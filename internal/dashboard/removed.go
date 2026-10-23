package dashboard

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// removedHint is what every key on a removed row says except the two that work
const removedHint = "removed: D deletes it for good, enter signs it back in"

type lineKind int

const (
	lineAccount lineKind = iota
	lineBlank
	lineHeader
	lineRemoved
)

// listLine is one row of the accounts pane: idx points into the snapshot
// Accounts or Removed, depending on the kind
type listLine struct {
	kind lineKind
	idx  int
}

// selectable lines are the ones the cursor can rest on
func (l listLine) selectable() bool { return l.kind == lineAccount || l.kind == lineRemoved }

// lineID is the account the line stands for, empty for a blank or a header
func (m model) lineID(l listLine) string {
	switch l.kind {
	case lineAccount:
		return m.snap.Accounts[l.idx].ID
	case lineRemoved:
		return m.snap.Removed[l.idx].ID
	}
	return ""
}

// listLines is the accounts that match the filter, then the Removed section
// when any removed account matches it
func (m model) listLines() []listLine {
	var out []listLine
	for _, i := range m.visible() {
		out = append(out, listLine{lineAccount, i})
	}
	f := strings.ToLower(m.filter)
	var gone []listLine
	for i, r := range m.snap.Removed {
		if f == "" || strings.Contains(strings.ToLower(r.Label), f) {
			gone = append(gone, listLine{lineRemoved, i})
		}
	}
	if len(gone) == 0 {
		return out
	}
	if len(out) > 0 {
		out = append(out, listLine{kind: lineBlank})
	}
	out = append(out, listLine{kind: lineHeader})
	return append(out, gone...)
}

// order is the ids the cursor visits, top to bottom
func (m model) order() []string {
	var ids []string
	for _, l := range m.listLines() {
		if l.selectable() {
			ids = append(ids, m.lineID(l))
		}
	}
	return ids
}

// selectedRemoved is the removed account under the cursor, if that is where
// it is
func (m model) selectedRemoved() (Removed, bool) {
	for _, l := range m.listLines() {
		if l.kind == lineRemoved && m.snap.Removed[l.idx].ID == m.sel {
			return m.snap.Removed[l.idx], true
		}
	}
	return Removed{}, false
}

func (m model) removedSelected() bool {
	_, ok := m.selectedRemoved()
	return ok
}

// removedRow is a dimmed row: the label and the recorded email after it
func removedRow(g glyphs, r Removed, selected bool, w int) rich {
	mark, name := " ", dim(r.Label)
	if selected {
		mark, name = g.cursor, bold(r.Label)
	}
	row := rich{dim(mark), gap(1), name}
	if r.Email != "" {
		row = append(row, gap(2), dim(r.Email))
	}
	return row.ellipsize(w, g.ellipsis)
}

// removedKey handles an action key on a removed row: D and enter work, the
// rest says why not
func (m model) removedKey(r Removed, k string) (model, tea.Cmd) {
	switch {
	case k == "enter":
		return m.working(m.launch(r.ID, true))
	case k == "D" && m.focus == paneAccounts:
		a := Account{ID: r.ID, Label: r.Label, Email: r.Email}
		a.Profile.Folder = r.Folder
		return m.askDelete(a), nil
	case k == "D":
		return m.say("Press tab to pick an account, then D to delete it.", false)
	}
	return m.say(removedHint, false)
}

// removedPane is the right pane of a removed row: its record and what can be
// done with it
func (m model) removedPane(g geom, r Removed) []string {
	email, folder := r.Email, r.Folder
	if email == "" {
		email = "none recorded"
	}
	if folder == "" {
		folder = "none"
	}
	var rows []rich
	for _, it := range [][2]string{{"Label", r.Label}, {"Email", email}, {"Folder", folder}, {"Status", "removed"}} {
		rows = append(rows, hang(rich{dim(it[0])}, profileLabelW, rich{txt(it[1])}, g.bodyW)...)
	}
	rows = append(rows, nil)
	rows = append(rows, wrap(rich{dim(removedHint)}, g.bodyW)...)
	return m.th.box(boxSpec{title: r.Label, rows: rows, w: g.rightW, h: g.paneH, padL: 1, padR: 1, focused: m.focus == paneDetail})
}

// removedHints is the footer on a removed row
var removedHints = []hint{
	{"enter", "sign in", 9}, {"D", "delete", 8}, {"tab", "pane", 5}, {"/", "find", 4}, {"?", "keys", 7}, {"q", "quit", 6},
}
