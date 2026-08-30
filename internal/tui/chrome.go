package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// header renders the logo line, account count, active-account meter, status dot,
// and version.
func (m dashModel) header() string {
	left := titleStyle.Render(string(rune(0x25C6)) + " clalt")
	if act := m.active(); act != nil {
		left += "   " + activeStyle.Render(act.Name) + " " + gradedMeter(liveFiveFrac(act), 10)
	}
	right := dimStyle.Render(pluralAccounts(len(m.data.Accounts)) + "   " + m.data.Version)
	return padBetween(left, right, m.viewWidth())
}

func (m dashModel) tabBar() string {
	var parts []string
	for i, t := range tabs {
		if i == m.tab {
			parts = append(parts, tabSelStyle.Render(t))
		} else {
			parts = append(parts, tabStyle.Render(t))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// footer is contextual: it shows the actions relevant to the current tab (or the
// active mode) plus the universal navigation keys.
func (m dashModel) footer() string {
	switch {
	case m.mode == "filter":
		return infoStyle.Render("/ filter: ") + textStyle.Render(m.query) + cursorStyle.Render("_") +
			dimStyle.Render("   enter apply   esc clear")
	case m.mode == "palette":
		return dimStyle.Render("up/down select   enter run   esc cancel")
	case m.confirming:
		return lipglossFg(colWarn).Bold(true).Render("delete "+m.confirmFor+" ? ") +
			dimStyle.Render("y = confirm, any other key = cancel")
	case m.flash != "":
		return lipglossFg(colWarn).Render(m.flash)
	}
	extra := map[string]string{
		"overview": "enter/r run   / filter",
	}[tabs[m.tab]]
	if extra != "" {
		extra += "      "
	}
	return dimStyle.Render(extra) + dimStyle.Render(string(rune(0x25CF))+" help ") + helpKey.Render("?") +
		dimStyle.Render("   quit ") + helpKey.Render("q")
}

func (m dashModel) helpView() string {
	line := func(k, d string) string { return "  " + helpKey.Render(padRight(k, 16)) + dimStyle.Render(d) + "\n" }
	var b strings.Builder
	b.WriteString(sectionStyle.Render("KEYBINDINGS") + "\n\n")
	b.WriteString(dimStyle.Render("navigation") + "\n")
	b.WriteString(line("left/right h l", "switch tabs"))
	b.WriteString(line("up/down j k", "select account"))
	b.WriteString("\n" + dimStyle.Render("actions") + "\n")
	b.WriteString(line("enter/r", "run selected account, keychain-free (isolated)"))
	b.WriteString(line("d", "delete the selected account (with confirm)"))
	b.WriteString(line("/", "filter accounts    : command palette"))
	b.WriteString("\n" + dimStyle.Render("general") + "\n")
	b.WriteString(line("?", "toggle this help"))
	b.WriteString(line("q", "quit"))
	b.WriteString("\n" + dimStyle.Render("press any key to close"))
	return b.String()
}
