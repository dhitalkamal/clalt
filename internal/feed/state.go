// Package feed is what claude's hooks and statusline tell clalt: the state of
// each running session and the rate limits of each account, kept as small
// files that readers may find missing or stale.
package feed

import "time"

// State is where a session is in its turn.
type State string

// the states of a session. needs-you is blocked on a decision of the user;
// idle is the pause between replies.
const (
	StateStarting State = "starting"
	StateBusy     State = "busy"
	StateIdle     State = "idle"
	StateNeedsYou State = "needs-you"
	StateEnded    State = "ended"
)

// SessionState is feed/sessions/<handle>.json.
type SessionState struct {
	Handle     string    `json:"handle"`
	Account    string    `json:"account"`
	State      State     `json:"state"`
	Since      time.Time `json:"since"`
	Reason     string    `json:"reason,omitempty"`
	LastError  string    `json:"lastError,omitempty"`
	Model      string    `json:"model,omitempty"`
	Cwd        string    `json:"cwd,omitempty"`
	ContextPct *float64  `json:"contextPct,omitempty"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Window is one rate limit window of an account.
type Window struct {
	UsedPct  float64   `json:"usedPct"`
	ResetsAt time.Time `json:"resetsAt"`
}

// Expired says whether the window has reset since it was seen.
func (w Window) Expired(now time.Time) bool { return !w.ResetsAt.After(now) }

// AccountLimits is feed/accounts/<account id>.json.
type AccountLimits struct {
	Account    string    `json:"account"`
	FiveHour   *Window   `json:"fiveHour,omitempty"`
	SevenDay   *Window   `json:"sevenDay,omitempty"`
	ObservedAt time.Time `json:"observedAt"`
}

func (s *SessionState) setState(st State, now time.Time) {
	if s.State != st {
		s.State, s.Since = st, now
	}
}
