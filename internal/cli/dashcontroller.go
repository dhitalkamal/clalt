package cli

import (
	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/profile"
	"github.com/dhitalkamal/clalt/internal/tui"
)

// App implements tui.Controller so the dashboard can run in-place management
// actions without the tui package importing cli. Teardown actions (run/login/add)
// are still returned to cmdDashboard as a tui.Action after the alt-screen closes.

// Data reloads the full dashboard payload.
func (a *App) Data(live bool) (tui.Data, error) { return a.buildDashboardData(live) }

// Rename delegates to the account store.
func (a *App) Rename(old, newName string) error { return a.Store.Rename(old, newName) }

// SetBudget sets an account's daily/weekly spend caps (USD).
func (a *App) SetBudget(name string, daily, weekly float64) error {
	return a.Store.SetBudget(name, daily, weekly)
}

// Logout forgets an account's stored credential (unconditional, like `clalt logout`).
func (a *App) Logout(name string) error { return a.Store.Delete(name) }

// Remove deletes an account slot, guarding the active account (like `clalt remove`).
func (a *App) Remove(name string) error { return a.Store.Remove(name) }

// Refresh rotates an account's token now.
func (a *App) Refresh(name string) error {
	_, err := a.refreshAccount(name)
	return err
}

// KillSession terminates a live run session and prunes its record.
func (a *App) KillSession(pid int) error { return profile.Kill(config.SessionsDir(), pid) }
