package dashboard

import (
	tea "charm.land/bubbletea/v2"
)

// ctrl+c always quits. the help overlay, the filter and the two questions own
// the keyboard while they are open.
func (m model) onKey(msg tea.KeyPressMsg) (model, tea.Cmd) {
	k := msg.String()
	switch {
	case k == "ctrl+c":
		return m, tea.Quit
	case m.entry.open:
		return m.entryKey(msg)
	case m.confirm.open:
		return m.confirmKey(k)
	case m.help:
		return m.helpKey(k)
	case m.filtering:
		return m.filterKey(msg)
	}
	return m.normalKey(k)
}

func (m model) helpKey(k string) (model, tea.Cmd) {
	switch k {
	case "esc", "?":
		m.help = false
	case "q":
		return m, tea.Quit
	case "up", "k":
		m.helpAt = max(m.helpAt-1, 0)
	case "down", "j":
		m.helpAt = min(m.helpAt+1, m.th.helpLayout(m.w, m.h).maxAt())
	}
	return m, nil
}

func (m model) filterKey(msg tea.KeyPressMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering, m.filter = false, ""
		return m.refilter(), nil
	case "enter":
		m.filtering = false
	case "backspace":
		if n := len(m.filter); n > 0 {
			m.filter = m.filter[:n-1]
			m = m.refilter()
		}
	case "up":
		return m.moveSel(-1), nil
	case "down":
		return m.moveSel(1), nil
	default:
		m = m.typeFilter(msg.Text)
	}
	return m, nil
}

// the filter takes printable ascii only, which keeps ascii mode ascii
func (m model) typeFilter(s string) model {
	for _, r := range s {
		if r >= ' ' && r <= '~' && (m.filter != "" || r != ' ') {
			m.filter += string(r)
		}
	}
	return m.refilter()
}

func (m model) normalKey(k string) (model, tea.Cmd) {
	switch k {
	case "q":
		return m, tea.Quit
	case "esc":
		switch {
		case m.drillFocused():
			m = m.closeDrill()
		case m.filter != "":
			m.filter = ""
			m = m.refilter()
		}
	case "backspace":
		if m.drillFocused() {
			m = m.closeDrill()
		}
	case "?":
		m.help, m.helpAt = true, 0
	case "/":
		m.filtering = true
		m.focus = paneAccounts
	case "a":
		m.opt.ASCII = !m.opt.ASCII
		m.th.g = glyphsFor(m.opt.ASCII)
	case "tab", "shift+tab":
		m.focus = 1 - m.focus
	case "]":
		m = m.setView((int(m.view) + 1) % len(tabNames))
	case "[":
		m = m.setView((int(m.view) + len(tabNames) - 1) % len(tabNames))
	case "1", "2", "3", "4", "5":
		m = m.setView(int(k[0] - '1'))
	case "up", "k":
		m = m.move(-1)
	case "down", "j":
		m = m.move(1)
	case "left", "h":
		m = m.histStep(-1)
	case "right", "l":
		m = m.histStep(1)
	case "enter":
		return m.enterKey()
	case "r", "d":
		return m.accountKey(k)
	case "s", "e", "x":
		return m.onAction(k)
	}
	return m, nil
}

// onPaste feeds pasted text to whichever input is open
func (m model) onPaste(s string) model {
	switch {
	case m.entry.open:
		return m.typeEntry(s)
	case m.filtering:
		return m.typeFilter(s)
	}
	return m
}
