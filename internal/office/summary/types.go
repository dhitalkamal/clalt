// Package summary builds the daily summary and the usage history of the
// office on demand from Claude Code's transcripts and the usage code. nothing
// is stored: every reply is computed from the files, bounded in files and
// bytes, and cached for a short time by the service.
package summary

// Version is the contract version of both replies.
const Version = 1

// Count is a name with how often it happened.
type Count struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Failure is a tool call that came back as an error, grouped by tool and text.
type Failure struct {
	Tool  string `json:"tool"`
	Text  string `json:"text"`
	Count int    `json:"count"`
}

// Stats is what one account or one project did in a day.
type Stats struct {
	Sessions      int       `json:"sessions"`
	Tokens        int64     `json:"tokens"`
	Cost          float64   `json:"cost"`
	ActiveMinutes int       `json:"activeMinutes"`
	ToolCalls     int       `json:"toolCalls"`
	Subagents     []Count   `json:"subagents"`
	Tools         []Count   `json:"tools"`
	Files         []Count   `json:"files"`
	FailureCount  int       `json:"failureCount"`
	Failures      []Failure `json:"failures"`
}

// ProjectOut is one project folder's stats.
type ProjectOut struct {
	Name string `json:"name"`
	Stats
}

// AccountOut is one account's stats and its projects.
type AccountOut struct {
	ID       string       `json:"id"`
	Label    string       `json:"label"`
	Projects []ProjectOut `json:"projects"`
	Stats
}

// Summary is the reply of GET /api/summary.
type Summary struct {
	V         int          `json:"v"`
	Day       string       `json:"day"`
	From      string       `json:"from"`
	To        string       `json:"to"`
	Truncated bool         `json:"truncated"`
	Totals    Stats        `json:"totals"`
	Accounts  []AccountOut `json:"accounts"`
}

// HistoryDay is one account's day. the percents are null unless a reading is
// known: only today has one, the feed keeps no history of readings.
type HistoryDay struct {
	Day          string   `json:"day"`
	Tokens       int64    `json:"tokens"`
	Cost         float64  `json:"cost"`
	Messages     int      `json:"messages"`
	Peak5hTokens int64    `json:"peak5hTokens"`
	Peak7dTokens int64    `json:"peak7dTokens"`
	Peak5hPct    *float64 `json:"peak5hPct"`
	Peak7dPct    *float64 `json:"peak7dPct"`
}

// HistoryAccount is one account's days, oldest first, one per day asked for.
type HistoryAccount struct {
	ID    string       `json:"id"`
	Label string       `json:"label"`
	Days  []HistoryDay `json:"days"`
}

// History is the reply of GET /api/history.
type History struct {
	V         int              `json:"v"`
	Days      int              `json:"days"`
	From      string           `json:"from"`
	To        string           `json:"to"`
	Truncated bool             `json:"truncated"`
	Accounts  []HistoryAccount `json:"accounts"`
}
