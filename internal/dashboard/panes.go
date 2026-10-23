package dashboard

import "fmt"

func (m model) filterRow() rich {
	slash := dim("/")
	if m.filtering {
		slash = bold("/")
	}
	text := txt(m.filter)
	if m.filter == "" {
		text = dim("type to filter")
	}
	return rich{slash, gap(1), text}
}

func (m model) accountsPane(g geom) []string {
	padR, rowW, wide := 2, rowWide, true
	if g.mode == modeNarrow {
		padR, rowW, wide = 1, rowNarrow, false
	}
	var rows []rich
	switch {
	case !m.have && m.loadErr != nil:
		rows = []rich{{bad("Load failed.")}}
	case !m.have:
		rows = []rich{{dim("Loading...")}}
	case len(m.snap.Accounts) == 0 && len(m.snap.Removed) == 0:
		rows = []rich{{dim("No accounts yet.")}}
	default:
		rows = m.accountRows(g, wide, rowW)
	}
	return m.th.box(boxSpec{title: "Accounts", rows: rows, w: g.leftW, h: g.paneH, padL: 1, padR: padR, focused: m.focus == paneAccounts})
}

func (m model) accountRows(g geom, wide bool, rowW int) []rich {
	var rows []rich
	filtered := m.filtering || m.filter != ""
	if filtered {
		rows = append(rows, m.filterRow(), nil)
	}
	lines := m.listLines()
	shown := m.listRoom(g, len(lines))
	off := clampInt(m.listOff, 0, max(len(lines)-shown, 0))
	for _, l := range lines[off : off+shown] {
		switch l.kind {
		case lineAccount:
			a := m.snap.Accounts[l.idx]
			rows = append(rows, accountRow(m.th.g, a, m.now(), a.ID == m.sel, wide, rowW))
		case lineBlank:
			rows = append(rows, nil)
		case lineHeader:
			rows = append(rows, rich{gap(2), dim("Removed")})
		case lineRemoved:
			r := m.snap.Removed[l.idx]
			rows = append(rows, removedRow(m.th.g, r, r.ID == m.sel, rowW))
		}
	}
	if hidden := len(lines) - shown; hidden > 0 {
		rows = append(rows, rich{gap(2), dim(fmt.Sprintf("+%d more", hidden))})
	}
	if len(lines) == 0 {
		rows = append(rows, rich{gap(2), dim("no match")})
	}
	if hint := roomHint(m.snap.Accounts); hint != "" && !filtered {
		lines := wrap(rich{dim(hint)}, rowW-2)
		if room := g.paneH - 2 - len(rows) - 1; room >= len(lines) {
			rows = append(rows, nil)
			for _, l := range lines {
				rows = append(rows, cat(rich{gap(2)}, l))
			}
		}
	}
	return rows
}

// plainPane is the right pane when there is no account to show
func (m model) plainPane(g geom, title string, rows []rich) []string {
	return m.th.box(boxSpec{title: title, rows: rows, w: g.rightW, h: g.paneH, padL: 1, padR: 1, focused: m.focus == paneDetail})
}

func (m model) detailPane(g geom) []string {
	switch {
	case !m.have && m.loadErr != nil:
		var rows []rich
		for _, l := range wrap(rich{bad("Could not load accounts: " + firstLine(m.loadErr.Error()))}, g.bodyW) {
			rows = append(rows, l)
		}
		rows = append(rows, nil, rich{dim("Trying again in the background.")})
		return m.plainPane(g, "Load failed", rows)
	case !m.have:
		return m.plainPane(g, "clalt", []rich{{dim("Reading your accounts...")}})
	case len(m.snap.Accounts) == 0 && len(m.snap.Removed) == 0:
		return m.plainPane(g, "No accounts", []rich{prompt("Add one with ", "clalt login <name>", ".")})
	}
	a, ok := m.selected()
	if r, gone := m.selectedRemoved(); !ok && gone {
		return m.removedPane(g, r)
	}
	if !ok {
		return m.plainPane(g, "No account", []rich{{dim("No account matches the filter.")}, {dim("Press esc to clear it.")}})
	}
	c := m.content(a, g)
	vh := max(g.viewH-len(c.head), 0)
	scroll := clampInt(m.scroll, 0, max(len(c.body)-vh, 0))
	rows := []rich{tabRow(m.th.g, tabNames, int(m.view), g.bodyW), nil}
	rows = append(rows, c.head...)
	rows = append(rows, c.body[scroll:min(scroll+vh, len(c.body))]...)
	var marks []bool
	if t := thumbRows(len(c.body), vh, scroll); t != nil {
		marks = append(make([]bool, 2+len(c.head)), t...)
	}
	return m.th.box(boxSpec{title: a.Label, rows: rows, w: g.rightW, h: g.paneH, padL: 1, padR: 1, focused: m.focus == paneDetail, thumb: marks})
}
