package office

import (
	"sort"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
)

// Version is the contract version of the snapshot.
const Version = 1

// maxSubagentsPerSession bounds what one session can put in the snapshot.
const maxSubagentsPerSession = 50

// Snapshot is what GET /api/snapshot returns.
type Snapshot struct {
	V        int          `json:"v"`
	Accounts []AccountOut `json:"accounts"`
	Sessions []SessionOut `json:"sessions"`
	Team     []TeamOut    `json:"team"`
	Reviews  ReviewsOut   `json:"reviews"`
}

// AccountOut is an account with its limits, by label only.
type AccountOut struct {
	ID     string    `json:"id"`
	Label  string    `json:"label"`
	Limits LimitsOut `json:"limits"`
	// Forecast is when the 5h window fills at the current pace, see forecastOf.
	Forecast ForecastOut `json:"forecast"`
}

// LimitsOut holds the two windows, null when unknown, and when they were read.
type LimitsOut struct {
	FiveHour *WindowOut `json:"fiveHour"`
	SevenDay *WindowOut `json:"sevenDay"`
	ReadAt   *time.Time `json:"readAt"`
}

// WindowOut is one rate limit window.
type WindowOut struct {
	UsedPct  float64   `json:"usedPct"`
	ResetsAt time.Time `json:"resetsAt"`
}

// SessionOut is one running session. Project is the folder name only.
type SessionOut struct {
	Handle         string     `json:"handle"`
	Account        string     `json:"account"`
	Project        string     `json:"project"`
	Model          string     `json:"model"`
	State          string     `json:"state"`
	NeedsYouReason string     `json:"needsYouReason"`
	StartedAt      time.Time  `json:"startedAt"`
	Subagents      []Subagent `json:"subagents"`
	// Actions says what the page may offer for this session, see ActionsOut.
	Actions ActionsOut `json:"actions"`
}

// ActionsOut is what can be done to a session from the page. Why is the reason
// when some of it cannot.
type ActionsOut struct {
	Move  bool   `json:"move"`
	Focus bool   `json:"focus"`
	Stop  bool   `json:"stop"`
	Why   string `json:"why"`
}

// outsideWhy is why a plain claude session cannot be controlled.
const outsideWhy = "started outside clalt: use clalt run to control it"

// Build reads the sources once and returns the snapshot. every string in it is
// cleaned, and lists are never nil so they encode as [].
func Build(src Sources) (Snapshot, error) {
	now := time.Now
	if src.Now != nil {
		now = src.Now
	}
	snap := Snapshot{V: Version, Accounts: []AccountOut{}, Sessions: []SessionOut{}, Team: []TeamOut{}}
	accts, err := src.Accounts.Accounts()
	if err != nil {
		return Snapshot{}, err
	}
	for _, a := range accts {
		out := AccountOut{ID: noEmail(clean(a.ID, maxName)), Label: noEmail(clean(a.Label, maxName))}
		if l, ok := src.Limits.Limits(a.ID); ok {
			out.Limits = limitsOut(l, now())
		}
		snap.Accounts = append(snap.Accounts, out)
	}
	sort.SliceStable(snap.Accounts, func(i, j int) bool {
		a, b := snap.Accounts[i], snap.Accounts[j]
		if a.Label != b.Label {
			return a.Label < b.Label
		}
		return a.ID < b.ID
	})
	list, err := src.Sessions.Sessions()
	if err != nil {
		return Snapshot{}, err
	}
	sort.SliceStable(list, func(i, j int) bool {
		if !list[i].StartedAt.Equal(list[j].StartedAt) {
			return list[i].StartedAt.Before(list[j].StartedAt)
		}
		return list[i].Handle < list[j].Handle
	})
	var cwds []string
	seen := map[string]bool{}
	for _, s := range list {
		snap.Sessions = append(snap.Sessions, sessionOut(s, src.Subagents))
		// the team reads each project folder once, by its real path
		if s.Cwd != "" && !seen[s.Cwd] {
			seen[s.Cwd] = true
			cwds = append(cwds, s.Cwd)
		}
	}
	var defs []AgentDef
	if src.Team != nil {
		defs = src.Team.Agents(cwds)
	}
	snap.Team = teamOut(defs, snap.Sessions)
	snap.Reviews, snap.Team = withReviews(src.Reviews, snap.Team)
	snap.fillForecasts(src, now())
	return snap, nil
}

// limitsOut drops a window that has reset since it was read: its figure
// describes a window that is over.
func limitsOut(l Limits, now time.Time) LimitsOut {
	out := LimitsOut{FiveHour: windowOut(l.FiveHour, now), SevenDay: windowOut(l.SevenDay, now)}
	if !l.ReadAt.IsZero() {
		at := l.ReadAt
		out.ReadAt = &at
	}
	return out
}

func windowOut(w *Window, now time.Time) *WindowOut {
	if w == nil || !w.ResetsAt.After(now) {
		return nil
	}
	return &WindowOut{UsedPct: w.UsedPct, ResetsAt: w.ResetsAt}
}

func sessionOut(s Session, subs SubagentSource) SessionOut {
	out := SessionOut{
		Handle: clean(s.Handle, maxName), Account: noEmail(clean(s.Account, maxName)), Project: clean(folderName(s.Cwd), maxName),
		Model: clean(s.Model, maxName), State: clean(s.State, 20), StartedAt: s.StartedAt, Subagents: []Subagent{},
	}
	if out.Account == "" {
		out.Account = "unknown"
	}
	if out.Project == "" {
		out.Project = "unknown"
	}
	out.Actions = s.Can
	if feed.IsExt(s.Handle) {
		out.Actions = ActionsOut{Why: outsideWhy}
	}
	// the reason is the notification type of the hook, and only means something while the session waits
	if s.State == "needs-you" {
		out.NeedsYouReason = clean(s.Reason, 40)
	}
	if subs != nil {
		list := subs.Subagents(s.Handle)
		if len(list) > maxSubagentsPerSession {
			list = list[:maxSubagentsPerSession]
		}
		for _, sub := range list {
			sub.ID, sub.Type, sub.Label, sub.State = clean(sub.ID, maxName), clean(sub.Type, maxName), clean(sub.Label, maxLabel), clean(sub.State, 20)
			out.Subagents = append(out.Subagents, sub)
		}
		sort.SliceStable(out.Subagents, func(i, j int) bool {
			a, b := out.Subagents[i], out.Subagents[j]
			if !a.StartedAt.Equal(b.StartedAt) {
				return a.StartedAt.Before(b.StartedAt)
			}
			return a.ID < b.ID
		})
	}
	return out
}

// fillForecasts gives each account with a live session its forecast.
func (s *Snapshot) fillForecasts(src Sources, now time.Time) {
	live := map[string]bool{}
	for _, x := range s.Sessions {
		if x.State != "ended" {
			live[x.Account] = true
		}
	}
	for i := range s.Accounts {
		a := &s.Accounts[i]
		if l, ok := src.Limits.Limits(a.ID); ok {
			a.Forecast = forecastOf(l, live[a.ID], now)
		}
	}
}
