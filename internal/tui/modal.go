package tui

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

// modalStep is one field the modal collects.
type modalStep struct {
	prompt  string
	numeric bool
}

// inputModal is a small multi-step text-input prompt, reused for rename, budget,
// login (new account name), and add (name for the current login).
type inputModal struct {
	active    bool
	kind      string // rename, budget, login, add
	name      string // target account (rename/budget)
	steps     []modalStep
	step      int
	value     string
	collected []string
}

// confirmState is a y/N confirmation, reused for remove, logout, and kill.
type confirmState struct {
	active bool
	kind   string // remove, logout, kill
	label  string
	name   string
	pid    int
}

func newModal(kind, name string, steps []modalStep) inputModal {
	return inputModal{active: true, kind: kind, name: name, steps: steps}
}

// updateModal handles keys while a text-input modal is open.
func (m dashModel) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.modal = inputModal{}
		return m, nil
	case tea.KeyEnter:
		m.modal.collected = append(m.modal.collected, m.modal.value)
		m.modal.value = ""
		m.modal.step++
		if m.modal.step < len(m.modal.steps) {
			return m, nil
		}
		return m.commitModal()
	case tea.KeyBackspace:
		if n := len(m.modal.value); n > 0 {
			m.modal.value = m.modal.value[:n-1]
		}
		return m, nil
	default:
		if len(msg.Runes) > 0 {
			r := string(msg.Runes)
			if !m.modal.curStep().numeric || isNumeric(r) {
				m.modal.value += r
			}
		}
		return m, nil
	}
}

// commitModal runs the modal's action once all fields are collected. Teardown
// actions (login/add) return an Action and quit; the rest delegate to the
// controller in place.
func (m dashModel) commitModal() (tea.Model, tea.Cmd) {
	mod := m.modal
	m.modal = inputModal{}
	switch mod.kind {
	case "rename":
		newName := mod.collected[0]
		if newName == "" {
			m.flash = "rename needs a new name"
			return m, nil
		}
		return m, doAction(func() error { return m.ctrl.Rename(mod.name, newName) }, "renamed "+mod.name, "rename")
	case "budget":
		daily := parseUSD(mod.collected[0])
		weekly := parseUSD(mod.collected[1])
		return m, doAction(func() error { return m.ctrl.SetBudget(mod.name, daily, weekly) },
			"budget set for "+mod.name, "budget")
	case "login":
		if mod.collected[0] == "" {
			m.flash = "login needs an account name"
			return m, nil
		}
		m.action = Action{Kind: "login", Account: mod.collected[0]}
		return m, tea.Quit
	case "add":
		if mod.collected[0] == "" {
			m.flash = "add needs a name"
			return m, nil
		}
		m.action = Action{Kind: "add", Account: mod.collected[0]}
		return m, tea.Quit
	}
	return m, nil
}

func (mod inputModal) curStep() modalStep {
	if mod.step < 0 || mod.step >= len(mod.steps) {
		return modalStep{}
	}
	return mod.steps[mod.step]
}

// parseUSD reads a dollar amount; blank or unparseable means 0 (clears the cap).
func parseUSD(s string) float64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

func isNumeric(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && r != '.' {
			return false
		}
	}
	return true
}
