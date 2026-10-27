// Package office serves the live 3D office: a read-only snapshot of the
// accounts, running sessions and subagents, and the embedded page that draws
// them. it listens on loopback only and wants a random token on every request.
package office

import (
	"time"
)

// Account is one account as the office labels it. the label is display only.
type Account struct {
	ID    string
	Label string
}

// Window is one rate limit window of an account.
type Window struct {
	UsedPct  float64
	ResetsAt time.Time
}

// Limits is what the feed knows of an account. a nil window is unknown, which
// is not 0 percent. a zero ReadAt means there is no reading at all.
type Limits struct {
	FiveHour *Window
	SevenDay *Window
	ReadAt   time.Time
}

// Session is one running session before it is cleaned for the snapshot.
type Session struct {
	Handle    string
	Account   string
	Cwd       string
	Model     string
	State     string
	Reason    string
	StartedAt time.Time
}

// Subagent is one subagent of a session. it is also the snapshot's own shape,
// see the contract in docs/plans/office.md. State is "running" or "done" and
// EndedAt is nil while it runs.
type Subagent struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Label     string     `json:"label"`
	State     string     `json:"state"`
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt"`
}

// AccountSource lists the accounts to show.
type AccountSource interface {
	Accounts() ([]Account, error)
}

// SessionSource lists the running sessions, plain claude ones included.
type SessionSource interface {
	Sessions() ([]Session, error)
}

// LimitSource reads the limits of an account. false means no reading.
type LimitSource interface {
	Limits(accountID string) (Limits, bool)
}

// SubagentSource lists the subagents of a session by its handle.
type SubagentSource interface {
	Subagents(handle string) []Subagent
}

// Sources is everything Build reads. Now is the clock, time.Now when nil.
type Sources struct {
	Accounts  AccountSource
	Sessions  SessionSource
	Limits    LimitSource
	Subagents SubagentSource
	Team      TeamSource
	Reviews   ReviewSource
	Now       func() time.Time
}
