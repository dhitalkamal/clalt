// Package live is the dashboard's source and actions on real data: the
// accounts in state.json, what claude says about their sign-ins, and the usage
// in the transcripts. what lives in other packages of clalt (cli, the session
// registry and feed of step 3a, Kiro Crew) comes in through the interfaces in
// this file, with defaults that return nothing.
package live

import (
	"context"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/claudecli"
	"github.com/dhitalkamal/clalt/internal/dashboard"
	"github.com/dhitalkamal/clalt/internal/usage"
)

// SignIn is how an account stands, judged from what claude reports for its
// folder against what clalt recorded when it signed in.
type SignIn struct {
	OK     bool
	Reason claudecli.Reason // why not, when it is not OK
	Text   string           // the reason in words, empty when OK
	Email  string
	Org    string
	Plan   string
}

// Host is what the adapters need from the rest of clalt. cli implements it with
// its own functions, so the dashboard follows the rules of the commands.
type Host interface {
	// CheckSignIn asks claude who is signed in to the account's folder.
	CheckSignIn(acc account.Account) (SignIn, error)
	// ClaudeVersion is the version of the claude that clalt drives. an error
	// says why it cannot be used, such as a version that is too old.
	ClaudeVersion() (string, error)
	// LinksInSync says whether the folder is linked to the shared claude home.
	LinksInSync(folder string) (bool, error)
	// Usage reads the transcripts and gives the records by account id. what no
	// account has is under usage.Unattributed.
	Usage() (map[string][]usage.Record, error)
	// SignOut signs the account out of claude and records it.
	SignOut(acc account.Account) error
	// Run and Login are the code of `clalt run` and `clalt login` for the
	// account, to run with the terminal released.
	Run(acc account.Account) dashboard.Launch
	Login(acc account.Account) dashboard.Launch
}

// Window is one rate limit window as the feed reports it.
type Window struct {
	Percent  int
	ResetsAt time.Time
}

// Limits are the two windows of an account and when they were read.
type Limits struct {
	Five, Seven Window
	ObservedAt  time.Time
}

// Reading is one reading of the 5h window, for the chart.
type Reading struct {
	At          time.Time
	FivePercent int
}

// Feed is what the session feed of step 3a knows about limits.
type Feed interface {
	// Limits are the latest readings of the account. false means none yet.
	Limits(ctx context.Context, accountID string) (Limits, bool, error)
	// Readings are the 5h readings since a time, oldest first.
	Readings(ctx context.Context, accountID string, since time.Time) ([]Reading, error)
	// Installed says whether the feed hooks are installed in the account.
	Installed(accountID string) bool
}

// SessionState is what the feed says a session is doing.
type SessionState string

// the states of the feed
const (
	StateStarting SessionState = "starting"
	StateBusy     SessionState = "busy"
	StateIdle     SessionState = "idle"
	StateNeedsYou SessionState = "needs-you"
	StateEnded    SessionState = "ended"
)

// Session is one running session: a registry record and its feed state.
type Session struct {
	Handle          string
	AccountID       string
	ClaudeSessionID string
	State           SessionState
	Reason          string
	Model           string
	Cwd             string
	StartedAt       time.Time
	UpdatedAt       time.Time
}

// Sessions is the session registry of step 3a. Stop must check again, when it
// is called, the pid, the start time and the process identity of the session
// before it signals anything, and refuse when they do not match: the dialog
// that asked may hold a handle that went stale while it was open.
type Sessions interface {
	Running(ctx context.Context) ([]Session, error)
	Stop(ctx context.Context, accountID, handle string) error
}

// Gateway is a Kiro Crew gateway of an account (step 6).
type Gateway struct {
	AccountID string
	Addr      string
}

// Gateways lists the running Kiro Crew gateways.
type Gateways interface {
	Running(ctx context.Context) ([]Gateway, error)
}

// NoFeed, NoSessions and NoGateways are the defaults: nothing is known yet.
type (
	NoFeed     struct{}
	NoSessions struct{}
	NoGateways struct{}
)

// Limits has no readings.
func (NoFeed) Limits(context.Context, string) (Limits, bool, error) { return Limits{}, false, nil }

// Readings has no history.
func (NoFeed) Readings(context.Context, string, time.Time) ([]Reading, error) { return nil, nil }

// Installed is false.
func (NoFeed) Installed(string) bool { return false }

// Running lists nothing.
func (NoSessions) Running(context.Context) ([]Session, error) { return nil, nil }

// Stop says it cannot, as a message and not a failure.
func (NoSessions) Stop(context.Context, string, string) error {
	return dashboard.Notice("Stopping sessions is not available yet.")
}

// Running lists nothing.
func (NoGateways) Running(context.Context) ([]Gateway, error) { return nil, nil }
