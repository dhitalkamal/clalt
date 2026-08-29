// Package account manages named Claude credential slots: secrets live in the
// macOS Keychain under the "clalt" service, non-secret metadata and the active
// pointer live in state.json, and switching swaps the active slot into Claude
// Code's live store.
package account

import "time"

// KeychainService is the namespace for per-account credential secrets.
const KeychainService = "clalt"

// Meta is the non-secret info shown in listings and the dashboard.
type Meta struct {
	Email      string    `json:"email,omitempty"`
	Plan       string    `json:"plan,omitempty"`
	AddedAt    time.Time `json:"addedAt"`
	LastUsedAt time.Time `json:"lastUsedAt,omitempty"`
	// spend caps in USD (0 = unset); used for budget bars and alerts.
	DailyBudget  float64 `json:"dailyBudget,omitempty"`
	WeeklyBudget float64 `json:"weeklyBudget,omitempty"`
}

// Account is a slot plus whether it is currently active.
type Account struct {
	Name   string
	Meta   Meta
	Active bool
}
