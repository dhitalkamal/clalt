package live

import (
	"sort"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/dashboard"
	"github.com/dhitalkamal/clalt/internal/usage"
)

const historyDays = 30

// ledger is what the transcripts say, built once per scan and read on every
// load. it holds the shown history and no records.
type ledger struct {
	histories    map[string]dashboard.History // by account id
	lastIn       map[string]string            // folder of the newest record, by account id
	sessionCost  map[string]float64           // by claude session id, any account
	unattributed float64                      // cost in the window that no account has
}

type dayAcc struct {
	in, out, cacheRead, cacheWrite int64
	cost                           float64
	sessions                       map[string]bool
}

type actKey struct {
	day   int
	model string
}

type actAcc struct {
	tokens   int64
	cost     float64
	sessions map[string]bool
}

type projAcc struct {
	cwd      string
	newest   time.Time
	tokens   int64
	cost     float64
	sessions map[string]bool
	act      map[actKey]*actAcc
}

type modelAcc struct {
	tokens int64
	cost   float64
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// dayIndex counts the days back from today, rounding so a clock change does
// not move a record. outside the window it is -1.
func dayIndex(today, ts time.Time) int {
	if ts.IsZero() {
		return -1
	}
	d := int(today.Sub(startOfDay(ts.In(today.Location()))).Hours()/24 + 0.5)
	if d < 0 || d >= historyDays {
		return -1
	}
	return d
}

func buildLedger(byAccount map[string][]usage.Record, now time.Time, home string) *ledger {
	l := &ledger{
		histories: map[string]dashboard.History{}, lastIn: map[string]string{}, sessionCost: map[string]float64{},
	}
	today := startOfDay(now)
	for id, recs := range byAccount {
		if id == usage.Unattributed {
			for _, r := range recs {
				if dayIndex(today, r.Timestamp) >= 0 {
					l.unattributed += usage.CostOf(r)
					l.sessionCost[r.SessionID] += usage.CostOf(r)
				}
			}
			continue
		}
		h, last := accountHistory(recs, today, home, l.sessionCost)
		if len(h.Days) > 0 {
			l.histories[id] = h
		}
		if last != "" {
			l.lastIn[id] = last
		}
	}
	return l
}

// accountHistory sums one account's records over the window. sessionCost gets
// the cost of every session on the way.
func accountHistory(recs []usage.Record, today time.Time, home string, sessionCost map[string]float64) (dashboard.History, string) {
	var days [historyDays]dayAcc
	projects := map[string]*projAcc{}
	models := map[string]*modelAcc{}
	var newest time.Time
	var lastCwd string
	any := false
	for _, r := range recs {
		d := dayIndex(today, r.Timestamp)
		if d < 0 {
			continue
		}
		any = true
		cost, tokens := usage.CostOf(r), r.Billable()
		if r.SessionID != "" {
			sessionCost[r.SessionID] += cost
		}
		da := &days[d]
		da.in, da.out, da.cacheRead, da.cacheWrite, da.cost = da.in+r.Input, da.out+r.Output, da.cacheRead+r.CacheRead, da.cacheWrite+r.CacheCreate, da.cost+cost
		addSession(&da.sessions, r.SessionID)

		p := projects[r.Project]
		if p == nil {
			p = &projAcc{act: map[actKey]*actAcc{}}
			projects[r.Project] = p
		}
		if r.Cwd != "" && (p.cwd == "" || r.Timestamp.After(p.newest)) {
			p.cwd, p.newest = r.Cwd, r.Timestamp
		}
		p.tokens, p.cost = p.tokens+tokens, p.cost+cost
		addSession(&p.sessions, r.SessionID)
		model := modelName(r.Model)
		ak := actKey{d, model}
		a := p.act[ak]
		if a == nil {
			a = &actAcc{}
			p.act[ak] = a
		}
		a.tokens, a.cost = a.tokens+tokens, a.cost+cost
		addSession(&a.sessions, r.SessionID)

		m := models[model]
		if m == nil {
			m = &modelAcc{}
			models[model] = m
		}
		m.tokens, m.cost = m.tokens+tokens, m.cost+cost

		if r.Cwd != "" && r.Timestamp.After(newest) {
			newest, lastCwd = r.Timestamp, r.Cwd
		}
	}
	if !any {
		return dashboard.History{}, ""
	}
	h := dashboard.History{Days: make([]dashboard.Day, historyDays)}
	for i := range days {
		h.Days[i] = dashboard.Day{
			Label: today.AddDate(0, 0, -i).Format("Mon 2"), Input: days[i].in, Output: days[i].out,
			CacheRead: days[i].cacheRead, CacheWrite: days[i].cacheWrite, Cost: days[i].cost, Sessions: len(days[i].sessions),
		}
	}
	h.Projects = projectShares(projects, today, home)
	for name, m := range models {
		h.Models = append(h.Models, dashboard.Share{Name: name, Tokens: m.tokens, Cost: m.cost})
	}
	sortShares(h.Models)
	return h, shorten(lastCwd, home)
}

func addSession(set *map[string]bool, id string) {
	if id == "" {
		return
	}
	if *set == nil {
		*set = map[string]bool{}
	}
	(*set)[id] = true
}

func sortShares(s []dashboard.Share) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].Cost != s[j].Cost {
			return s[i].Cost > s[j].Cost
		}
		return s[i].Name < s[j].Name
	})
}

func projectShares(projects map[string]*projAcc, today time.Time, home string) []dashboard.Share {
	paths := map[string]string{}
	for key, p := range projects {
		paths[key] = shorten(p.cwd, home)
	}
	names := shortNames(paths)
	var out []dashboard.Share
	for key, p := range projects {
		s := dashboard.Share{Name: names[key], Tokens: p.tokens, Cost: p.cost, Sessions: len(p.sessions)}
		var keys []actKey
		for k := range p.act {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			a, b := keys[i], keys[j]
			switch {
			case a.day != b.day:
				return a.day < b.day
			case p.act[a].cost != p.act[b].cost:
				return p.act[a].cost > p.act[b].cost
			}
			return a.model < b.model
		})
		for _, k := range keys {
			a := p.act[k]
			s.Activity = append(s.Activity, dashboard.Activity{
				Day: today.AddDate(0, 0, -k.day).Format("Mon 2"), Model: k.model, Sessions: len(a.sessions), Tokens: a.tokens, Cost: a.cost,
			})
		}
		out = append(out, s)
	}
	sortShares(out)
	return out
}

// shorten writes the home folder as ~
func shorten(path, home string) string {
	if home != "" && (path == home || strings.HasPrefix(path, home+"/")) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

// shortNames gives every key the fewest trailing folders of its path that tell
// it from the others. a key with no path keeps its own name, which is also what
// breaks a tie that no number of folders can.
func shortNames(paths map[string]string) map[string]string {
	out := map[string]string{}
	for depth := 1; depth <= 64 && len(out) < len(paths); depth++ {
		counts := map[string]int{}
		cand := map[string]string{}
		for key, p := range paths {
			if _, done := out[key]; done {
				continue
			}
			cand[key] = lastFolders(p, depth)
			counts[cand[key]]++
		}
		for key, c := range cand {
			if c != "" && counts[c] == 1 && !taken(out, c) {
				out[key] = c
			}
		}
	}
	for key := range paths {
		if _, ok := out[key]; !ok {
			out[key] = key
		}
	}
	return out
}

func taken(names map[string]string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

func lastFolders(path string, n int) string {
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts[max(len(parts)-n, 0):], "/")
}
