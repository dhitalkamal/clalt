// Package wrapper keeps a claude session going across account moves. it starts
// claude, and at a pause moves the session to another account: it ends claude and
// starts `claude --resume` under the other account in the same terminal. a move
// is asked for with a request file (clalt move) or decided by auto-switch.
//
// everything it needs from outside is an interface defined here: the session
// registry, the feed, the accounts, the claude process and the terminal. the
// adapters to the real ones live in package cli.
package wrapper

import (
	"os"
	"time"

	"github.com/dhitalkamal/clalt/internal/pick"
)

// Record is a running session, as the registry keeps it.
type Record struct {
	Handle          string
	Account         string // account id
	Folder          string
	WrapperPID      int
	ClaudePID       int
	ClaudeSessionID string
	Cwd             string
	Terminal        string
	StartedAt       time.Time
}

// Change is what the registry is told after claude is started: the zero fields
// are left as they are.
type Change struct {
	Account   string
	Folder    string
	ClaudePID int
}

// Registry is the session registry.
type Registry interface {
	List() ([]Record, error)
	// Dir is where the records live. move requests are written next to them.
	Dir() string
	Update(handle string, c Change) error
}

// State is what the feed says a session is doing.
type State int

// the states of a session. only Idle is a pause.
const (
	Unknown State = iota
	Starting
	Busy
	Idle
	NeedsYou
	Ended
)

// SessionInfo is the feed's view of one session.
type SessionInfo struct {
	State State
	// LimitHit is set when claude stopped because the account's limit was reached.
	LimitHit  bool
	Cwd       string
	UpdatedAt time.Time
}

// Feed is the live state of sessions and the limits of accounts.
type Feed interface {
	Session(handle string) (SessionInfo, bool)
	Limits(accountID string) (pick.Limits, bool)
}

// Prepared is an account that is ready to run a session.
type Prepared struct {
	Label  string
	Folder string
	// Notes are what getting the account ready had to say. claude owns the
	// terminal while it runs, so they are printed when it is gone.
	Notes []string
}

// Accounts is what the wrapper needs to know about accounts.
type Accounts interface {
	Candidates() ([]pick.Candidate, error)
	// Prepare checks the rules and the sign-in of an account and links its
	// folder, as `clalt run` does, and returns where it is. the error is for
	// the user.
	Prepare(id, cwd string) (Prepared, error)
}

// Process is a running claude.
type Process interface {
	PID() int
	// Wait returns when claude has ended, with what it ended with.
	Wait() error
	// Terminate asks claude to end by itself and save what it has.
	Terminate() error
	Kill() error
	Signal(os.Signal) error
	// EndedByTerminate says whether what Wait returned looks like the end that
	// Terminate asked for. an end that does not is the user quitting.
	EndedByTerminate(err error) bool
}

// Launcher starts claude for an account folder.
type Launcher interface {
	Start(folder string, args []string) (Process, error)
}

// Terminal puts the terminal back the way it was before claude. forced says that
// claude was killed, and so could not do it itself.
type Terminal interface {
	Restore(forced bool)
}
