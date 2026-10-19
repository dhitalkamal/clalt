// package dashboard is the focus-panes terminal dashboard. it owns its view
// types and two ports, Source and Actions, and knows nothing about where the
// data comes from.
package dashboard

import "time"

// Snapshot is one consistent read of everything the dashboard shows. totals are
// not repeated in it: today, the week, the 30 day sparkline and the budget
// spend all come from History.Days, so the views cannot disagree.
type Snapshot struct {
	Accounts []Account
	Warnings []string // about the read as a whole, shown under the header
}

// Account is one profile. Actions address it by ID, which is unique and stable.
// the label is display only and may change.
type Account struct {
	ID         string
	Label      string
	Email      string
	Org        string
	Plan       string // empty while signed out
	SignedIn   bool
	Problem    string // what is wrong with the account, empty when nothing is
	Login      Login
	LastUsedAt time.Time // zero when never used
	LastIn     string    // folder of the last session
	Limits     Limits
	Budget     Budget
	Sessions   []Session
	History    History
	Profile    Profile
}

// Login is when the account signed in and when the 30 day cap forces a new
// sign in. a zero time means the source does not know.
type Login struct {
	SignedInAt time.Time
	DueAt      time.Time
}

// Limits holds the 5h and 7d windows plus the readings behind the chart.
type Limits struct {
	Five     Window
	Seven    Window
	ReadAt   time.Time
	Readings []float64 // 5h percent, evenly spaced across Days, oldest first
	Days     []string  // one label per day the readings cover, oldest first
}

// Level is how worried a line should look.
type Level int

const (
	LevelOK Level = iota
	LevelWarn
	LevelBad
)

// Window is one rate limit window. the forecast is data and the dashboard
// writes the sentences, so every view describes it the same way.
type Window struct {
	Percent   int // used so far, 0 to 100
	ResetsAt  time.Time
	Projected int       // percent used at ResetsAt if the pace holds, 100 or more means it runs out first
	RunsOutAt time.Time // when it reaches 100 at this pace, zero unless that is before ResetsAt
}

// Budget holds spend caps in dollars, zero meaning no cap.
type Budget struct {
	Daily  float64
	Weekly float64
}

// History is the usage record behind the totals. Days run newest first, the
// first one is today and the first seven make the week. tokens counted as the
// headline are input plus output, cache is shown beside them.
type History struct {
	Days     []Day
	Projects []Share
	Models   []Share
}

// Day is one calendar day of usage.
type Day struct {
	Label      string // "Wed 15"
	Input      int64
	Output     int64
	CacheRead  int64
	CacheWrite int64
	Cost       float64 // estimated dollars
	Sessions   int
}

// Share is one project or model over the last 30 days.
type Share struct {
	Name     string
	Tokens   int64 // input plus output
	Cost     float64
	Sessions int        // projects only
	Activity []Activity // projects only, newest first, what the drill-in lists
}

// Activity is one day of one project on one model.
type Activity struct {
	Day      string // "Wed 15"
	Model    string
	Sessions int
	Tokens   int64 // input plus output
	Cost     float64
}

// SessionState is what a running session is doing.
type SessionState string

const (
	StateWorking SessionState = "working"
	StateWaiting SessionState = "waiting"
	StateIdle    SessionState = "idle"
)

// Session is one running claude session.
type Session struct {
	ID          string
	State       SessionState
	Folder      string
	Branch      string
	Model       string
	Effort      string
	ContextUsed int64
	ContextSize int64
	StartedAt   time.Time
	Added       int
	Removed     int
	Cost        float64
}

// Profile holds the facts shown on the profile view.
type Profile struct {
	Folder         string
	ClaudeVersion  string
	SharedSettings bool // settings, skills and MCP shared with ~/.claude
	FeedInstalled  bool // the limits feed hook is installed
}
