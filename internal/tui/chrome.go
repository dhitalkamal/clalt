package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// header renders the logo + version on the left and how many accounts are
// signed in (plus a refresh spinner) on the right. what could not be loaded is
// in between, cut to the room there is, since the numbers below it are then not
// to be trusted.
func (m dashModel) header() string {
	left := titleStyle.Render(string(rune(0x25C6))+" clalt") + dimStyle.Render("  "+m.data.Version)

	var right strings.Builder
	if m.refreshing {
		right.WriteString(dimStyle.Render(spinFrames[m.spin] + " "))
	}
	signedIn := 0
	for _, a := range m.data.Accounts {
		if a.SignedIn {
			signedIn++
		}
	}
	right.WriteString(dimStyle.Render("signed in: " + itoa(signedIn) + "/" + itoa(len(m.data.Accounts))))
	if warning := oneLine(m.data.Warning); warning != "" {
		const minRoom = 12
		if room := m.viewWidth() - 1 - lipgloss.Width(left) - lipgloss.Width(right.String()) - 4; room >= minRoom {
			left += "  " + lipglossFg(colWarn).Render(trunc(warning, room))
		}
	}
	return padBetween(left, right.String(), m.viewWidth()-1)
}

// footer is context-sensitive: modes and overlays take priority, otherwise it
// shows the keys relevant to the focused panel.
func (m dashModel) footer() string {
	switch {
	case m.modal.active:
		prompt := m.modal.curStep().prompt
		return clampLine(infoStyle.Render(prompt+": ")+textStyle.Render(m.modal.value)+
			cursorStyle.Render("_")+dimStyle.Render("   enter ok   esc cancel"), m.viewWidth()-1)
	case m.confirm.active:
		return clampLine(lipglossFg(colWarn).Bold(true).Render(m.confirm.label+" ")+
			dimStyle.Render("y = confirm, any other key = cancel"), m.viewWidth()-1)
	case m.mode == "filter":
		return clampLine(infoStyle.Render("/ filter: ")+textStyle.Render(m.query)+cursorStyle.Render("_")+
			dimStyle.Render("   enter apply   esc clear"), m.viewWidth()-1)
	case m.mode == "palette":
		return dimStyle.Render("up/down select   enter run   esc cancel")
	case m.flash != "":
		return clampLine(lipglossFg(colWarn).Render(m.flash), m.viewWidth()-1)
	}

	var keys string
	switch m.focus {
	case focusSessions:
		keys = "tab focus  k kill  : cmd  ? help  q quit"
	default:
		keys = "tab focus  enter/r run  e budget  R rename  n login  d remove  / : ? q"
	}
	return clampLine(dimStyle.Render(keys), m.viewWidth()-1)
}

// helpView renders the full keybinding reference shown by ?.
func (m dashModel) helpView(innerW int) string {
	line := func(k, d string) string {
		return "  " + helpKey.Render(padRight(k, 16)) + dimStyle.Render(d) + "\n"
	}
	var b strings.Builder
	b.WriteString(sectionStyle.Render("KEYBINDINGS") + "\n\n")
	b.WriteString(dimStyle.Render("global") + "\n")
	b.WriteString(line("tab / shift+tab", "cycle panel focus"))
	b.WriteString(line("up/down j k", "move selection in the focused panel"))
	b.WriteString(line("/", "filter accounts     : command palette"))
	b.WriteString(line("? / q", "toggle help / quit"))
	b.WriteString("\n" + dimStyle.Render("accounts panel") + "\n")
	b.WriteString(line("enter / r", "run the selected account"))
	b.WriteString(line("n", "sign in an account (a new one or an existing one)"))
	b.WriteString(line("e / R", "set budget / rename (modal input)"))
	b.WriteString(line("l", "sign out (confirm), history stays"))
	b.WriteString(line("d / x", "remove the selected account (confirm), history stays"))
	b.WriteString("\n" + dimStyle.Render("sessions panel") + "\n")
	b.WriteString(line("k", "kill the selected session (confirm)"))
	b.WriteString("\n" + dimStyle.Render("press any key to close"))
	return b.String()
}
