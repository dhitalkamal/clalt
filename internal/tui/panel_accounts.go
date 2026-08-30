package tui

import "strings"

// renderAccounts draws the accounts panel: one row per account with a 5h
// utilization percent (color-graded) and a near-expiry marker. The selected row
// is highlighted when the panel is focused.
func (m dashModel) renderAccounts(innerW, innerH int) string {
	focused := m.focus == focusAccounts
	accts := m.visibleAccounts()

	var b strings.Builder
	if len(accts) == 0 {
		b.WriteString(dimStyle.Render(trunc("no accounts match "+m.query, innerW)))
	}
	start, end := windowRange(m.sel, len(accts), innerH)
	for i := start; i < end; i++ {
		b.WriteString(m.accountRow(accts[i], i == m.sel, focused, innerW) + "\n")
	}
	return panelBox("Accounts", innerW, innerH, b.String(), focused)
}

// accountRow formats one account line to exactly innerW columns.
func (m dashModel) accountRow(a AccountView, selected, focused bool, innerW int) string {
	frac := liveFiveFrac(&a)
	flag := " "
	if a.NearExpiry {
		flag = lipglossFg(colErr).Render("!")
	}
	cur := "  "
	if selected {
		cur = cursorStyle.Render("> ")
	}
	nameW := innerW - 8 // cur(2) + name + space + pct(4) + flag(1) - space(1)
	if nameW < 3 {
		nameW = 3
	}
	name := a.Name
	if a.Active {
		name = name + " *"
	}
	nameCell := padRight(trunc(name, nameW), nameW)
	if a.Active {
		nameCell = activeStyle.Render(nameCell)
	}
	row := cur + nameCell + " " + lipglossFg(gradedColor(frac)).Render(padLeft(pct(frac), 4)) + flag
	if selected && focused {
		// pre-pad then highlight so the selection bar spans the full panel width
		row = rowSelStyle.Render(padRight(clampLine(row, innerW), innerW))
	}
	return row
}

// windowRange returns [start,end) of a list windowed to rows entries, keeping the
// selection visible near the middle.
func windowRange(sel, n, rows int) (int, int) {
	if rows >= n {
		return 0, n
	}
	if rows < 1 {
		rows = 1
	}
	start := sel - rows/2
	if start < 0 {
		start = 0
	}
	if start+rows > n {
		start = n - rows
	}
	return start, start + rows
}
