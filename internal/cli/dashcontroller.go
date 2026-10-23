package cli

import (
	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/profile"
	"github.com/dhitalkamal/clalt/internal/tui"
)

// App implements tui.Controller so the dashboard can run in-place management
// actions without the tui package importing cli. Teardown actions (run, login)
// are still returned to cmdDashboard as a tui.Action after the alt-screen
// closes. the dashboard knows accounts by label, and nothing here prints: the
// dashboard owns the screen while an action runs.

// every error that leaves these methods is clean: the dashboard draws it, and
// the text comes from claude and from the file system.

// Data reloads the full dashboard payload.
func (a *App) Data() (tui.Data, error) { return a.buildDashboardData() }

// Rename changes an account's label.
func (a *App) Rename(old, newName string) error {
	acc, err := a.findVisible(old)
	if err != nil {
		return cleanErr(err)
	}
	return cleanedOrNil(a.relabel(acc.Name, newName))
}

// SetBudget sets an account's daily/weekly spend caps (USD).
func (a *App) SetBudget(name string, daily, weekly float64) error {
	acc, err := a.findVisible(name)
	if err != nil {
		return cleanErr(err)
	}
	return cleanedOrNil(a.Store.SetBudget(acc.Name, daily, weekly))
}

// Logout signs an account out, like `clalt logout`.
func (a *App) Logout(name string) error {
	acc, err := a.findVisible(name)
	if err != nil {
		return cleanErr(err)
	}
	return cleanedOrNil(a.signOut(acc))
}

// Remove signs an account out and hides it, like `clalt remove`.
func (a *App) Remove(name string) error {
	acc, err := a.findVisible(name)
	if err != nil {
		return cleanErr(err)
	}
	return cleanedOrNil(a.removeAccount(acc))
}

// KillSession terminates a live run session and prunes its record.
func (a *App) KillSession(pid int) error {
	return cleanedOrNil(profile.Kill(config.SessionsDir(), pid))
}

func cleanedOrNil(err error) error {
	if err == nil {
		return nil
	}
	return cleanErr(err)
}
