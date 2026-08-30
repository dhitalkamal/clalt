package tui

import tea "github.com/charmbracelet/bubbletea"

// trigger is the single dispatch point shared by keybindings and the command
// palette. Teardown actions (run/login/add) set an Action and quit; input actions
// open a modal; destructive actions open a confirm; refresh runs in place.
func (m dashModel) trigger(kind, name string, pid int) (tea.Model, tea.Cmd) {
	m.mode = "" // leave the palette if we came from it
	switch kind {
	case "run":
		m.action = Action{Kind: "run", Account: name}
		return m, tea.Quit
	case "login":
		m.modal = newModal("login", "", []modalStep{{prompt: "new account name"}})
		return m, nil
	case "add":
		m.modal = newModal("add", "", []modalStep{{prompt: "name for the current login"}})
		return m, nil
	case "rename":
		m.modal = newModal("rename", name, []modalStep{{prompt: "new name for " + name}})
		return m, nil
	case "budget":
		m.modal = newModal("budget", name, []modalStep{
			{prompt: "daily cap USD (0 clears)", numeric: true},
			{prompt: "weekly cap USD (0 clears)", numeric: true},
		})
		return m, nil
	case "logout":
		m.confirm = confirmState{active: true, kind: "logout", name: name,
			label: "forget stored credential for " + name + " ?"}
		return m, nil
	case "remove":
		if a := m.accountByName(name); a != nil && a.Active {
			m.flash = "cannot remove the active account"
			return m, nil
		}
		m.confirm = confirmState{active: true, kind: "remove", name: name,
			label: "delete account " + name + " ?"}
		return m, nil
	case "refresh":
		return m, doAction(func() error { return m.ctrl.Refresh(name) }, "refreshed "+name, "refresh")
	case "kill":
		m.confirm = confirmState{active: true, kind: "kill", name: name, pid: pid,
			label: "kill session " + name + " ?"}
		return m, nil
	}
	return m, nil
}

// updateConfirm handles the y/N confirmation overlay.
func (m dashModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := m.confirm
	if k := msg.String(); k == "y" || k == "Y" {
		m.confirm = confirmState{}
		switch c.kind {
		case "remove":
			return m, doAction(func() error { return m.ctrl.Remove(c.name) }, "removed "+c.name, "remove")
		case "logout":
			return m, doAction(func() error { return m.ctrl.Logout(c.name) }, "logged out "+c.name, "logout")
		case "kill":
			return m, doAction(func() error { return m.ctrl.KillSession(c.pid) }, "killed "+c.name, "kill")
		}
	}
	m.confirm = confirmState{} // any other key cancels
	return m, nil
}

// doAction runs a controller call off the main loop and reports the outcome as an
// actionDoneMsg (which sets the flash and triggers a data reload).
func doAction(fn func() error, okMsg, verb string) tea.Cmd {
	return func() tea.Msg {
		if err := fn(); err != nil {
			return actionDoneMsg{flash: verb + " failed: " + err.Error()}
		}
		return actionDoneMsg{flash: okMsg}
	}
}

// accountByName looks up an account in the current data by name.
func (m dashModel) accountByName(name string) *AccountView {
	for i := range m.data.Accounts {
		if m.data.Accounts[i].Name == name {
			return &m.data.Accounts[i]
		}
	}
	return nil
}
