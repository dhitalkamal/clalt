// Package account manages named Claude credential slots: secrets live in
// clalt's local encrypted vault under the "clalt" service namespace, non-secret
// metadata and the active pointer live in state.json, and switching swaps the
// active slot into Claude Code's live store.
package account

import "time"

// KeychainService is the vault namespace for per-account credential secrets.
// The name is historical: this used to be the macOS Keychain service string
// back when secrets lived in the OS Keychain; it is now just the key prefix
// used inside the local encrypted vault (keychain.EncFileStore).
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

	// added with thin profiles. the fields above are still written so the
	// v0.1.0 binary can read the file. an unset label means the id is the label.
	Label string `json:"label,omitempty"`
	Org   string `json:"org,omitempty"`
	// omitempty alone never drops a zero time.Time, omitzero does
	SignedInAt time.Time `json:"signedInAt,omitempty,omitzero"`
	Hidden     bool      `json:"hidden,omitempty"`
}

// Account is a slot plus whether it is currently active.
type Account struct {
	Name   string // the account id: the state.json key and the folder under accounts/
	Meta   Meta
	Active bool
}

// Label is what the user sees and types: Meta.Label, else the id.
func (a Account) Label() string { return labelOf(a.Name, a.Meta) }

func labelOf(id string, m Meta) string {
	if m.Label != "" {
		return m.Label
	}
	return id
}
