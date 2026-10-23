package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// updateFilter handles keys while the account filter (/) is open.
func (m dashModel) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode, m.query, m.sel = "", "", 0
	case tea.KeyEnter:
		m.mode = "" // keep the query applied to the list
	case tea.KeyBackspace:
		if len(m.query) > 0 {
			m.query = m.query[:len(m.query)-1]
		}
	case tea.KeyUp:
		if m.sel > 0 {
			m.sel--
		}
	case tea.KeyDown:
		if m.sel < len(m.visibleAccounts())-1 {
			m.sel++
		}
	default:
		if len(msg.Runes) > 0 {
			m.query += string(msg.Runes)
		}
	}
	if n := len(m.visibleAccounts()); m.sel >= n {
		m.sel = maxInt(0, n-1)
	}
	return m, nil
}

// updatePalette handles keys while the command palette (:) is open.
func (m dashModel) updatePalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cmds := m.paletteCommands()
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = ""
	case tea.KeyEnter:
		if m.palSel < len(cmds) {
			c := cmds[m.palSel]
			return m.trigger(c.kind, c.name, c.pid)
		}
	case tea.KeyBackspace:
		if len(m.query) > 0 {
			m.query = m.query[:len(m.query)-1]
			m.palSel = 0
		}
	case tea.KeyUp:
		if m.palSel > 0 {
			m.palSel--
		}
	case tea.KeyDown:
		if m.palSel < len(cmds)-1 {
			m.palSel++
		}
	default:
		if len(msg.Runes) > 0 {
			m.query += string(msg.Runes)
			m.palSel = 0
		}
	}
	return m, nil
}

// visibleAccounts applies the current name filter to the account list.
func (m dashModel) visibleAccounts() []AccountView {
	if m.query == "" {
		return m.data.Accounts
	}
	q := strings.ToLower(m.query)
	var out []AccountView
	for _, a := range m.data.Accounts {
		if strings.Contains(strings.ToLower(a.Name), q) {
			out = append(out, a)
		}
	}
	return out
}

// palCmd is one command-palette entry, dispatched via trigger.
type palCmd struct {
	label string
	kind  string
	name  string
	pid   int
}

// paletteCommands builds the (filtered) command list. It lists only the current
// verbs: run/login/rename/budget/logout/remove. The removed
// switch/next/auto/fallback/add/refresh verbs are intentionally absent.
func (m dashModel) paletteCommands() []palCmd {
	var all []palCmd
	all = append(all, palCmd{"login (new account)", "login", "", 0})
	for _, a := range m.data.Accounts {
		all = append(all,
			palCmd{"run " + a.Name, "run", a.Name, 0},
			palCmd{"rename " + a.Name, "rename", a.Name, 0},
			palCmd{"budget " + a.Name, "budget", a.Name, 0},
			palCmd{"logout " + a.Name, "logout", a.Name, 0},
			palCmd{"remove " + a.Name, "remove", a.Name, 0},
		)
	}
	if m.query == "" {
		return all
	}
	q := strings.ToLower(m.query)
	var out []palCmd
	for _, cd := range all {
		if strings.Contains(strings.ToLower(cd.label), q) {
			out = append(out, cd)
		}
	}
	return out
}

// paletteView renders the command palette body (shown inside the detail column).
func (m dashModel) paletteView(innerW int) string {
	var b strings.Builder
	b.WriteString(infoStyle.Render(": ") + textStyle.Render(m.query) + cursorStyle.Render("_") + "\n\n")
	cmds := m.paletteCommands()
	if len(cmds) == 0 {
		b.WriteString(dimStyle.Render("  no matching commands"))
		return b.String()
	}
	for i, cd := range cmds {
		if i >= 14 {
			b.WriteString(dimStyle.Render("  ..."))
			break
		}
		line := clampLine(cd.label, innerW-2)
		if i == m.palSel {
			b.WriteString(cursorStyle.Render("> ") + textStyle.Render(line) + "\n")
		} else {
			b.WriteString("  " + dimStyle.Render(line) + "\n")
		}
	}
	return b.String()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
