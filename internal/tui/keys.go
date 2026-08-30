package tui

import tea "github.com/charmbracelet/bubbletea"

// handleKey routes a keypress: overlays and modes get first refusal, then the
// global keys, then the keys specific to the focused panel.
func (m dashModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showHelp {
		m.showHelp = false // any key closes help
		return m, nil
	}
	if m.modal.active {
		return m.updateModal(msg)
	}
	if m.mode == "filter" {
		return m.updateFilter(msg)
	}
	if m.mode == "palette" {
		return m.updatePalette(msg)
	}
	if m.confirm.active {
		return m.updateConfirm(msg)
	}

	m.flash = ""
	switch msg.String() {
	case "tab":
		m.focus = (m.focus + 1) % focusCount
		return m, nil
	case "shift+tab":
		m.focus = (m.focus + focusCount - 1) % focusCount
		return m, nil
	case "?":
		m.showHelp = true
		return m, nil
	case "/":
		m.mode, m.query, m.focus = "filter", "", focusAccounts
		return m, nil
	case ":":
		m.mode, m.query, m.palSel = "palette", "", 0
		return m, nil
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	}

	switch m.focus {
	case focusAccounts:
		return m.keysAccounts(msg)
	case focusSessions:
		return m.keysSessions(msg)
	}
	return m, nil
}

// keysAccounts handles keys while the accounts panel is focused.
func (m dashModel) keysAccounts(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	a := m.active()
	switch msg.String() {
	case "up", "k":
		if m.sel > 0 {
			m.sel--
		}
	case "down", "j":
		if m.sel < len(m.visibleAccounts())-1 {
			m.sel++
		}
	case "enter", "r":
		if a != nil {
			return m.trigger("run", a.Name, 0)
		}
	case "n":
		return m.trigger("login", "", 0)
	case "a":
		return m.trigger("add", "", 0)
	case "e":
		if a != nil {
			return m.trigger("budget", a.Name, 0)
		}
	case "R":
		if a != nil {
			return m.trigger("rename", a.Name, 0)
		}
	case "l":
		if a != nil {
			return m.trigger("logout", a.Name, 0)
		}
	case "f":
		if a != nil {
			return m.trigger("refresh", a.Name, 0)
		}
	case "d", "x":
		if a != nil {
			return m.trigger("remove", a.Name, 0)
		}
	}
	return m, nil
}

// keysSessions handles keys while the sessions panel is focused. k kills the
// selected session (arrows move the selection here, since k is bound to kill).
func (m dashModel) keysSessions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		if m.sessSel > 0 {
			m.sessSel--
		}
	case "down", "j":
		if m.sessSel < len(m.data.Sessions)-1 {
			m.sessSel++
		}
	case "k":
		if s := m.selectedSession(); s != nil {
			return m.trigger("kill", s.Account, s.PID)
		}
	}
	return m, nil
}

// selectedSession is the highlighted session, or nil when there are none.
func (m dashModel) selectedSession() *SessionInfo {
	if m.sessSel < 0 || m.sessSel >= len(m.data.Sessions) {
		return nil
	}
	return &m.data.Sessions[m.sessSel]
}
