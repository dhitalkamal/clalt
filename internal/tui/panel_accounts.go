package tui

import "strings"

// renderAccounts draws the accounts panel: one row per account with a 5h
// utilization estimate (color-graded) and a mark for an account that is signed
// out or has to sign in again soon. The selected row is highlighted when the
// panel is focused.
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
	frac := fiveFrac(&a)
	flag := " "
	if a.needsLogin() {
		flag = lipglossFg(colErr).Render("!")
	}
	cur := "  "
	if selected {
		cur = cursorStyle.Render("> ")
	}
	nameW := innerW - 9 // cur(2) + name + space + estimate(5) + flag(1) - space(1)
	if nameW < 3 {
		nameW = 3
	}
	nameCell := padRight(trunc(a.Name, nameW), nameW)
	row := cur + nameCell + " " + lipglossFg(gradedColor(frac)).Render(padLeft(estimate(frac), 5)) + flag
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
