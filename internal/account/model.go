// Package account keeps what clalt knows about each account: its label, who
// signed in and when, and its spend caps, in state.json. there are no secrets
// here: claude keeps each account's login in the account's own folder.
package account

import "time"

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

// Account is one account of the state file. the state also keeps the active
// pointer v0.1.0 wrote, so that the old binary can still read the file, but
// nothing reads it here.
type Account struct {
	Name string // the account id: the state.json key and the folder under accounts/
	Meta Meta
}

// Label is what the user sees and types: Meta.Label, else the id.
func (a Account) Label() string { return labelOf(a.Name, a.Meta) }

func labelOf(id string, m Meta) string {
	if m.Label != "" {
		return m.Label
	}
	return id
}
