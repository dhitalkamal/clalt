package demo

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/dhitalkamal/clalt/internal/dashboard"
)

type actions struct{}

var _ dashboard.Actions = actions{}

// Actions only say what they would have done, as a dashboard.Notice. nothing
// runs and nothing changes.
func Actions() dashboard.Actions { return actions{} }

func (actions) Run(_ context.Context, id string) (*exec.Cmd, error) {
	return nil, dashboard.Notice("Demo: would start claude as " + id + ".")
}

func (actions) SignIn(_ context.Context, id string) (*exec.Cmd, error) {
	return nil, dashboard.Notice("Demo: would sign in to " + id + ".")
}

func (actions) Rename(_ context.Context, id, label string) error {
	return dashboard.Notice(fmt.Sprintf("Demo: would rename %s to %s.", id, label))
}

func (actions) SetBudget(_ context.Context, id string, daily, weekly float64) error {
	amount := func(v float64, unit, adjective string) string {
		if v <= 0 {
			return "no " + adjective + " cap"
		}
		return fmt.Sprintf("$%.2f a %s", v, unit)
	}
	if daily <= 0 && weekly <= 0 {
		return dashboard.Notice(fmt.Sprintf("Demo: would clear %s's budget.", id))
	}
	return dashboard.Notice(fmt.Sprintf("Demo: would set %s's budget to %s and %s.", id, amount(daily, "day", "daily"), amount(weekly, "week", "weekly")))
}

func (actions) SignOut(_ context.Context, id string) error {
	return dashboard.Notice("Demo: would sign out " + id + " (history kept).")
}

func (actions) StopSession(_ context.Context, id, sessionID string) error {
	return dashboard.Notice("Demo: would stop that session.")
}
