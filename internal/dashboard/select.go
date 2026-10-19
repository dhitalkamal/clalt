package dashboard

import "strings"

func (m model) geom() geom {
	return geometry(m.w, m.h, m.focus == paneDetail).shrunk(m.warnRows())
}

// warnRows is the one line under the header the warnings take, if any show
func (m model) warnRows() int {
	if m.have && len(m.snap.Warnings) > 0 {
		return 1
	}
	return 0
}

// visible lists the accounts that match the filter, by index into the snapshot
func (m model) visible() []int {
	f := strings.ToLower(m.filter)
	var out []int
	for i, a := range m.snap.Accounts {
		if f == "" || strings.Contains(strings.ToLower(a.Label), f) {
			out = append(out, i)
		}
	}
	return out
}

// listRoom is how many of n accounts the list shows. when they do not all fit,
// the last line of the pane says how many are left out.
func (m model) listRoom(g geom, n int) int {
	room := g.paneH - 2
	if m.filtering || m.filter != "" {
		room -= 2
	}
	if n <= room {
		return n
	}
	return max(room-1, 1)
}

// scrollList keeps the selected account inside the window of rows shown
func (m model) scrollList(g geom) model {
	vis := m.visible()
	shown := m.listRoom(g, len(vis))
	for pos, i := range vis {
		switch {
		case m.snap.Accounts[i].ID != m.sel:
		case pos < m.listOff:
			m.listOff = pos
		case pos >= m.listOff+shown:
			m.listOff = pos - shown + 1
		}
	}
	m.listOff = clampInt(m.listOff, 0, max(len(vis)-shown, 0))
	return m
}

func (m model) selected() (Account, bool) {
	for _, i := range m.visible() {
		if a := m.snap.Accounts[i]; a.ID == m.sel {
			return a, true
		}
	}
	return Account{}, false
}

// resetRight is the right pane starting over, which also closes the drill-in
func (m model) resetRight() model {
	m.rowCur, m.scroll, m.drill, m.sessID = 0, 0, drill{}, ""
	return m
}

// refilter keeps the selection if it still shows, else takes the first match
func (m model) refilter() model {
	vis := m.visible()
	for _, i := range vis {
		if m.snap.Accounts[i].ID == m.sel {
			return m
		}
	}
	if len(vis) > 0 {
		m.sel = m.snap.Accounts[vis[0]].ID
		m = m.resetRight()
	}
	return m
}

func (m model) setView(v int) model {
	m.view = viewID(v)
	return m.resetRight()
}

func (m model) histStep(d int) model {
	if m.focus != paneDetail || m.view != viewHistory || m.drill.open() {
		return m
	}
	m.hist = (m.hist + d + len(subNames)) % len(subNames)
	return m.resetRight()
}

func (m model) moveSel(d int) model {
	vis := m.visible()
	pos := -1
	for i, v := range vis {
		if m.snap.Accounts[v].ID == m.sel {
			pos = i
		}
	}
	if pos < 0 {
		return m
	}
	if next := clampInt(pos+d, 0, len(vis)-1); next != pos {
		m.sel = m.snap.Accounts[vis[next]].ID
		m = m.resetRight()
	}
	return m
}

// move steps the account list, or the cursor and scroll in the right pane
func (m model) move(d int) model {
	if m.focus == paneAccounts {
		return m.moveSel(d)
	}
	a, ok := m.selected()
	if !ok {
		return m
	}
	if c := m.content(a, m.geom()); c.rows > 0 {
		m.rowCur = clampInt(m.rowCur+d, 0, c.rows-1)
		if m.view == viewSessions && m.rowCur < len(a.Sessions) {
			m.sessID = a.Sessions[m.rowCur].ID
		}
	} else {
		m.scroll += d
	}
	return m
}

func (m model) content(a Account, g geom) content {
	c := viewCtx{g: m.th.g, w: g.bodyW, h: g.viewH, focused: m.focus == paneDetail, row: m.rowCur, sub: m.hist, drill: m.drill.project, now: m.now()}
	switch m.view {
	case viewSessions:
		return sessionsContent(c, a)
	case viewLimits:
		return limitsContent(c, a)
	case viewHistory:
		return historyContent(c, a)
	case viewProfile:
		return profileContent(c, a)
	}
	return overviewContent(c, a)
}

// synced keeps the cursor row and the scroll offset inside the content
func (m model) synced() model {
	g := m.geom()
	a, ok := m.selected()
	if m.drill.open() && !(ok && m.drill.holds(a)) {
		m = m.closeDrill()
	}
	if m.help {
		m.helpAt = clampInt(m.helpAt, 0, m.th.helpLayout(m.w, m.h).maxAt())
	}
	if g.mode == modeTooSmall {
		return m
	}
	m = m.scrollList(g)
	if g.rightW == 0 || !ok {
		return m
	}
	m = m.followSession(a)
	c := m.content(a, g)
	vh := max(g.viewH-len(c.head), 1)
	m.rowCur = clampInt(m.rowCur, 0, max(c.rows-1, 0))
	if c.rows > 0 {
		start := c.first + m.rowCur*c.stride
		if start < m.scroll {
			m.scroll = start
		}
		if end := start + c.span; end > m.scroll+vh {
			m.scroll = end - vh
		}
	}
	m.scroll = clampInt(m.scroll, 0, max(len(c.body)-vh, 0))
	return m
}

// followSession keeps the cursor on the session it was on when the list changes.
// one that has ended hands the cursor to whatever now sits in its row.
func (m model) followSession(a Account) model {
	if m.view != viewSessions || !a.SignedIn || len(a.Sessions) == 0 {
		m.sessID = ""
		return m
	}
	for i, s := range a.Sessions {
		if s.ID == m.sessID {
			m.rowCur = i
			return m
		}
	}
	m.rowCur = clampInt(m.rowCur, 0, len(a.Sessions)-1)
	m.sessID = a.Sessions[m.rowCur].ID
	return m
}
