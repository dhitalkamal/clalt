package dashboard

import (
	"fmt"
	"strconv"

	tea "charm.land/bubbletea/v2"
)

const maxAmountLen = 10

// dollars is an amount as typed, empty for no cap
func dollars(v float64) string {
	if v <= 0 {
		return ""
	}
	return fmt.Sprintf("%.2f", v)
}

func parseDollars(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v >= 1e7 {
		return 0, fmt.Errorf("not an amount: %q", s)
	}
	return v, nil
}

func (m model) openBudget(a Account) model {
	m.entry = entry{open: true, id: a.ID, label: a.Label, weeklyNow: a.Budget.Weekly, text: dollars(a.Budget.Daily)}
	return m
}

// typeAmount takes digits and a dot, nothing else
func (m model) typeAmount(s string) model {
	for _, r := range s {
		if (r >= '0' && r <= '9' || r == '.') && len(m.entry.text) < maxAmountLen {
			m.entry.text += string(r)
			m.entry.note = ""
		}
	}
	return m
}

func (m model) submitBudget() (model, tea.Cmd) {
	e := m.entry
	v, err := parseDollars(e.text)
	if err != nil {
		m.entry.note = "Enter an amount like 10.00."
		return m, nil
	}
	if e.step == 0 {
		m.entry.daily, m.entry.step, m.entry.text, m.entry.note = v, 1, dollars(e.weeklyNow), ""
		return m, nil
	}
	m.entry = entry{}
	act, ctx := m.act, m.ctx
	return m.working(m.actCmd("Saved the budget for "+e.label+".", func() error { return act.SetBudget(ctx, e.id, e.daily, v) }))
}
