package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhitalkamal/clalt/internal/account"
)

type pickerModel struct {
	items  []account.Account
	cursor int
	chosen string
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "enter":
		m.chosen = m.items[m.cursor].Name
		return m, tea.Quit
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m pickerModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("select an account to switch to") + "\n\n")
	for i, it := range m.items {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("> ")
		}
		label := it.Name
		if it.Meta.Email != "" {
			label += dimStyle.Render("  " + it.Meta.Email)
		}
		if it.Active {
			label = activeStyle.Render(label + "  (active)")
		}
		fmt.Fprintf(&b, "%s%s\n", cursor, label)
	}
	b.WriteString("\n" + dimStyle.Render("up/down to move, enter to select, q to cancel"))
	return b.String()
}

// PickAccount runs the interactive picker and returns the chosen account name,
// or empty string if the user cancelled.
func PickAccount(items []account.Account) (string, error) {
	m := pickerModel{items: items}
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return "", err
	}
	return final.(pickerModel).chosen, nil
}
