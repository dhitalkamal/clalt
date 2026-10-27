// package demo is a dashboard source and actions on made up data, for trying
// the dashboard without touching real accounts. everything it shows comes from
// one ledger of usage records, so the totals agree wherever they appear.
package demo

import (
	"sort"
	"time"

	"github.com/dhitalkamal/clalt/internal/dashboard"
)

const (
	opus   = "Opus 5.5"
	sonnet = "Sonnet 5.5"
)

// record is one day of one project on one model: how many sessions it stands
// for, their tokens, and what they cost in cents. a named session is one
// running session, which the sessions view looks up by its id.
type record struct {
	day        int // days before today, 0 is today
	project    string
	model      string
	session    string
	sessions   int
	input      int64
	output     int64
	cacheRead  int64
	cacheWrite int64
	cents      int
}

type ledger []record

// the rows in ledger_*.go were fitted to the approved frame numbers, so change
// them with the tests open
var ledgers = map[string]ledger{
	"robin":  robinRecords,
	"morgan": morganRecords,
	"casey":  caseyRecords,
	"jordan": jordanRecords,
}

const historyDays = 30

// startOfDay is midnight of the day now falls on
func startOfDay(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func dollars(cents int) float64 { return float64(cents) / 100 }

func (l ledger) forSession(id string) ledger {
	var out ledger
	for _, r := range l {
		if r.session == id {
			out = append(out, r)
		}
	}
	return out
}

func (l ledger) days(today time.Time) []dashboard.Day {
	cents := make([]int, historyDays)
	days := make([]dashboard.Day, historyDays)
	for i := range days {
		days[i].Label = today.AddDate(0, 0, -i).Format("Mon 2")
	}
	for _, r := range l {
		d := &days[r.day]
		d.Input += r.input
		d.Output += r.output
		d.CacheRead += r.cacheRead
		d.CacheWrite += r.cacheWrite
		d.Sessions += r.sessions
		cents[r.day] += r.cents
	}
	for i := range days {
		days[i].Cost = dollars(cents[i])
	}
	return days
}

// shares groups the ledger by project or model, biggest cost first
func (l ledger) shares(by func(record) string) []dashboard.Share {
	cents := map[string]int{}
	byName := map[string]*dashboard.Share{}
	for _, r := range l {
		name := by(r)
		s, ok := byName[name]
		if !ok {
			s = &dashboard.Share{Name: name}
			byName[name] = s
		}
		s.Tokens += r.input + r.output
		s.Sessions += r.sessions
		cents[name] += r.cents
	}
	out := make([]dashboard.Share, 0, len(byName))
	for name, s := range byName {
		s.Cost = dollars(cents[name])
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Cost != out[j].Cost {
			return out[i].Cost > out[j].Cost
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (l ledger) history(today time.Time) dashboard.History {
	if len(l) == 0 {
		return dashboard.History{}
	}
	projects := l.shares(func(r record) string { return r.project })
	for i := range projects {
		projects[i].Activity = l.activity(projects[i].Name, today)
	}
	return dashboard.History{
		Days:     l.days(today),
		Projects: projects,
		Models:   l.shares(func(r record) string { return r.model }),
	}
}

// activity is a project's rows, one per day and model, newest first and the
// costliest first within a day
func (l ledger) activity(project string, today time.Time) []dashboard.Activity {
	type key struct {
		day   int
		model string
	}
	sums := map[key]record{}
	for _, r := range l {
		if r.project != project {
			continue
		}
		k := key{r.day, r.model}
		sum := sums[k]
		sum.day, sum.model = r.day, r.model
		sum.sessions += r.sessions
		sum.input += r.input
		sum.output += r.output
		sum.cents += r.cents
		sums[k] = sum
	}
	rows := make([]record, 0, len(sums))
	for _, r := range sums {
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		switch {
		case a.day != b.day:
			return a.day < b.day
		case a.cents != b.cents:
			return a.cents > b.cents
		}
		return a.model < b.model
	})
	out := make([]dashboard.Activity, len(rows))
	for i, r := range rows {
		out[i] = dashboard.Activity{
			Day: today.AddDate(0, 0, -r.day).Format("Mon 2"), Model: r.model,
			Sessions: r.sessions, Tokens: r.input + r.output, Cost: dollars(r.cents),
		}
	}
	return out
}

// sessionCost is what a running session has cost, from its ledger row
func (l ledger) sessionCost(id string) float64 {
	cents := 0
	for _, r := range l.forSession(id) {
		cents += r.cents
	}
	return dollars(cents)
}
