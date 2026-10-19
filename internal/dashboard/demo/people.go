package demo

import (
	"time"

	"github.com/dhitalkamal/clalt/internal/dashboard"
)

// liveSession is what the ledger does not know about a running session. its
// cost comes from the ledger row with the same id.
type liveSession struct {
	id             string
	state          dashboard.SessionState
	folder, branch string
	model, effort  string
	used, size     int64
	running        time.Duration // how long it has been running at the load
	added, removed int
}

// person is an account's facts that are not usage: identity, login, limits,
// budget and the sessions running now
type person struct {
	id, email, org, plan string
	signedIn             bool
	signedFor            time.Duration // how long ago the login was made, the cap is 30 days after it
	lastUsed             time.Duration // how long ago, zero when never used
	lastIn               string
	five, seven          window
	readAge              time.Duration // how old the limits reading is at the load
	chart                [7][8]int     // 5h percent, eight readings a day for a week
	budget               dashboard.Budget
	live                 []liveSession
	sharedSettings       bool
}

// window is a rate limit window as the demo writes it, with offsets from the load
type window struct {
	percent, projected int
	resetsIn           time.Duration
	runsOutIn          time.Duration // zero when it does not run out before the reset
}

func (w window) at(now time.Time) dashboard.Window {
	out := dashboard.Window{Percent: w.percent, Projected: w.projected, ResetsAt: now.Add(w.resetsIn)}
	if w.runsOutIn > 0 {
		out.RunsOutAt = now.Add(w.runsOutIn)
	}
	return out
}

const (
	day          = 24 * time.Hour
	loginCapDays = 30
)

// the offsets below sit half a minute or half a day past the number they show,
// so the demo reads the same for a few seconds either side of the load
func (p person) account(now time.Time) dashboard.Account {
	a := dashboard.Account{ID: p.id, Label: p.id, Email: p.email, Org: p.org, Plan: p.plan, SignedIn: p.signedIn}
	if !p.signedIn {
		return a
	}
	l := ledgers[p.id]
	signedInAt := now.Add(-p.signedFor)
	a.Login = dashboard.Login{SignedInAt: signedInAt, DueAt: signedInAt.AddDate(0, 0, loginCapDays)}
	if p.lastUsed > 0 {
		a.LastUsedAt = now.Add(-p.lastUsed)
	}
	a.LastIn = p.lastIn
	a.Budget = p.budget
	today := startOfDay(now)
	a.History = l.history(today)
	a.Limits = dashboard.Limits{Five: p.five.at(now), Seven: p.seven.at(now), ReadAt: now.Add(-p.readAge), Readings: p.readings()}
	for i := 6; i >= 0; i-- {
		a.Limits.Days = append(a.Limits.Days, today.AddDate(0, 0, -i).Format("Mon"))
	}
	for _, s := range p.live {
		a.Sessions = append(a.Sessions, dashboard.Session{
			ID: s.id, State: s.state, Folder: s.folder, Branch: s.branch, Model: s.model, Effort: s.effort,
			ContextUsed: s.used, ContextSize: s.size, StartedAt: now.Add(-s.running), Added: s.added, Removed: s.removed,
			Cost: l.sessionCost(s.id),
		})
	}
	a.Profile = dashboard.Profile{
		Folder: "~/.config/clalt/profiles/" + p.id, ClaudeVersion: "2.1.296",
		SharedSettings: p.sharedSettings, FeedInstalled: true,
	}
	return a
}

func (p person) readings() []float64 {
	out := make([]float64, 0, 56)
	for _, day := range p.chart {
		for _, v := range day {
			out = append(out, float64(v))
		}
	}
	return out
}

var people = []person{
	{
		id: "robin", email: "robin@example.com", org: "Example", plan: "team plan", signedIn: true,
		signedFor: 7*day + 12*time.Hour, lastUsed: 4*time.Minute + 30*time.Second, lastIn: "~/Projects/personal/clalt",
		five:   window{percent: 26, projected: 135, resetsIn: 1*time.Hour + 52*time.Minute + 30*time.Second, runsOutIn: 1*time.Hour + 17*time.Minute + 30*time.Second},
		seven:  window{percent: 42, projected: 71, resetsIn: 39*time.Hour + 52*time.Minute + 30*time.Second},
		budget: dashboard.Budget{Daily: 10, Weekly: 50},
		chart: [7][8]int{
			{8, 22, 61, 74, 35, 52, 19, 6},
			{5, 18, 44, 68, 83, 41, 27, 9},
			{0, 3, 12, 20, 9, 4, 0, 0},
			{0, 0, 6, 15, 28, 22, 10, 3},
			{7, 25, 58, 79, 91, 46, 30, 12},
			{10, 31, 66, 88, 52, 71, 38, 14},
			{6, 20, 47, 72, 39, 55, 33, 26},
		},
		live: []liveSession{
			{id: "k-live-1", state: dashboard.StateWorking, folder: "~/Projects/personal/clalt", branch: "feat/dashboard-v2",
				model: opus, effort: "high", used: 312_000, size: 1_000_000, running: 2*time.Hour + 4*time.Minute + 30*time.Second, added: 156, removed: 23},
			{id: "k-live-2", state: dashboard.StateWaiting, folder: "~/Projects/work/web", branch: "develop",
				model: sonnet, effort: "medium", used: 48_000, size: 1_000_000, running: 40*time.Minute + 30*time.Second, added: 12, removed: 3},
		},
		sharedSettings: true,
	},
	{
		id: "morgan", email: "morgan@example.com", org: "Example", plan: "max plan", signedIn: true,
		signedFor: 28*day + 12*time.Hour, lastUsed: 3*time.Hour + 30*time.Minute, lastIn: "~/Projects/work/api",
		five:    window{percent: 22, projected: 41, resetsIn: 2*time.Hour + 17*time.Minute + 30*time.Second},
		seven:   window{percent: 30, projected: 57, resetsIn: 3*day + 6*time.Hour + 27*time.Minute + 30*time.Second},
		readAge: 2 * time.Minute,
		budget:  dashboard.Budget{Daily: 12, Weekly: 60},
		chart: [7][8]int{
			{4, 15, 33, 48, 25, 30, 12, 3},
			{6, 20, 41, 57, 66, 38, 18, 5},
			{0, 0, 0, 0, 0, 0, 0, 0},
			{0, 0, 0, 0, 0, 0, 0, 0},
			{5, 18, 36, 52, 31, 44, 20, 8},
			{9, 27, 54, 71, 83, 49, 25, 10},
			{3, 14, 29, 46, 38, 33, 17, 22},
		},
		sharedSettings: true,
	},
	{
		id: "casey", email: "casey@example.com", org: "Example", plan: "max plan", signedIn: true,
		signedFor: 11*day + 12*time.Hour, lastUsed: 11*time.Minute + 30*time.Second, lastIn: "~/Projects/work/api",
		five:   window{percent: 95, projected: 180, resetsIn: 48*time.Minute + 30*time.Second, runsOutIn: 5*time.Minute + 15*time.Second},
		seven:  window{percent: 80, projected: 97, resetsIn: 18*time.Hour + 12*time.Minute + 30*time.Second},
		budget: dashboard.Budget{Daily: 8, Weekly: 40},
		chart: [7][8]int{
			{10, 35, 72, 90, 64, 78, 40, 12},
			{8, 30, 66, 84, 95, 70, 33, 10},
			{0, 6, 18, 30, 12, 5, 0, 0},
			{0, 4, 14, 28, 36, 20, 9, 2},
			{12, 40, 78, 92, 97, 68, 38, 15},
			{15, 44, 80, 96, 99, 72, 50, 21},
			{18, 52, 85, 98, 70, 88, 91, 95},
		},
		live: []liveSession{
			{id: "a-live-1", state: dashboard.StateWaiting, folder: "~/Projects/work/api", branch: "feat/export",
				model: opus, effort: "high", used: 912_000, size: 1_000_000, running: time.Hour + 12*time.Minute + 30*time.Second, added: 88, removed: 41},
		},
		sharedSettings: true,
	},
	{
		id: "jordan", email: "jordan@example.com", org: "Example", plan: "pro plan", signedIn: true,
		signedFor: 19*day + 12*time.Hour, lastUsed: 48*time.Hour + 30*time.Minute, lastIn: "~/Projects/personal/site",
		five:    window{percent: 5, projected: 12, resetsIn: 4*time.Hour + 37*time.Minute + 30*time.Second},
		seven:   window{percent: 11, projected: 24, resetsIn: 5*day + 11*time.Hour + 22*time.Minute + 30*time.Second},
		readAge: 9 * time.Minute,
		chart: [7][8]int{
			{0, 0, 0, 0, 0, 0, 0, 0},
			{0, 0, 4, 9, 3, 0, 0, 0},
			{0, 0, 0, 0, 0, 0, 0, 0},
			{0, 0, 8, 14, 6, 2, 0, 0},
			{0, 5, 22, 31, 12, 3, 0, 0},
			{0, 0, 0, 0, 0, 0, 0, 0},
			{0, 0, 0, 0, 0, 0, 0, 5},
		},
	},
	{id: "harriet", email: "harriet@example.com", org: "Example"},
}
