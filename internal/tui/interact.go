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
			m.action = cmds[m.palSel].act
			return m, tea.Quit
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

// selectedAccount is the highlighted account within the visible (filtered) list.
func (m dashModel) selectedAccount() *AccountView {
	vis := m.visibleAccounts()
	if m.sel < 0 || m.sel >= len(vis) {
		return nil
	}
	return &vis[m.sel]
}

type palCmd struct {
	label string
	act   Action
}

// paletteCommands builds the (filtered) command list for the palette.
func (m dashModel) paletteCommands() []palCmd {
	var all []palCmd
	all = append(all, palCmd{"rotate to next account", Action{Kind: "rotate"}})
	for _, a := range m.data.Accounts {
		all = append(all, palCmd{"switch " + a.Name, Action{Kind: "switch", Account: a.Name}})
		all = append(all, palCmd{"run " + a.Name + " (parallel)", Action{Kind: "run", Account: a.Name}})
		if !a.Active {
			all = append(all, palCmd{"remove " + a.Name, Action{Kind: "remove", Account: a.Name}})
		}
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

// paletteView renders the command palette body.
func (m dashModel) paletteView() string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("COMMAND PALETTE") + "\n")
	b.WriteString(infoStyle.Render(": ") + textStyle.Render(m.query) + cursorStyle.Render("_") + "\n\n")
	cmds := m.paletteCommands()
	if len(cmds) == 0 {
		b.WriteString(dimStyle.Render("  no matching commands"))
		return b.String()
	}
	for i, cd := range cmds {
		if i >= 12 {
			b.WriteString(dimStyle.Render("  ..."))
			break
		}
		if i == m.palSel {
			b.WriteString(cursorStyle.Render("> ") + textStyle.Render(cd.label) + "\n")
		} else {
			b.WriteString("  " + dimStyle.Render(cd.label) + "\n")
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
