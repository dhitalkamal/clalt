package dashboard

import tea "charm.land/bubbletea/v2"

// drill is the project drill-in under History > Projects. while it is open the
// table is replaced by that project's activity, and row and scroll remember
// where the table was so back lands on the same row.
type drill struct {
	project string // empty when the table shows
	row     int
	scroll  int
}

func (d drill) open() bool { return d.project != "" }

// drillFocused is true while the drill-in is open and the right pane has the keys
func (m model) drillFocused() bool { return m.drill.open() && m.focus == paneDetail }

// canDrill is true when enter should open the project under the cursor
func (m model) canDrill(a Account) bool {
	return m.focus == paneDetail && m.view == viewHistory && m.hist == 1 && !m.drill.open() &&
		a.SignedIn && m.rowCur < len(a.History.Projects)
}

// holds is true while the account still has the project that is open
func (d drill) holds(a Account) bool {
	_, ok := findProject(a.History.Projects, d.project)
	return ok && a.SignedIn
}

func (m model) openDrill(a Account) model {
	m.drill = drill{project: a.History.Projects[m.rowCur].Name, row: m.rowCur, scroll: m.scroll}
	m.scroll = 0
	return m
}

// closeDrill goes back to the table with the cursor where it was
func (m model) closeDrill() model {
	d := m.drill
	m.drill = drill{}
	m.rowCur, m.scroll = d.row, d.scroll
	return m
}

// enterKey opens the project under the cursor, does nothing inside the
// drill-in, and means run everywhere else
func (m model) enterKey() (model, tea.Cmd) {
	a, ok := m.selected()
	switch {
	case ok && m.drillFocused():
		return m, nil
	case ok && m.canDrill(a):
		return m.openDrill(a), nil
	}
	return m.onAction("enter")
}
