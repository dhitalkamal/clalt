package usage

import (
	"sort"
	"time"
)

// PeriodStat is a headline summary for a set of records (e.g. today or lifetime).
type PeriodStat struct {
	Billable int64
	Input    int64
	Output   int64
	Cost     float64
	Messages int
}

// ModelUsage is per-model lifetime usage.
type ModelUsage struct {
	Model    string
	Billable int64
	Cost     float64
	Messages int
}

// ProjectUsage is per-project lifetime usage.
type ProjectUsage struct {
	Project  string
	Billable int64
	Messages int
}

// CompositionPct is the share of the grand token total by class (sums to ~1).
type CompositionPct struct {
	Input      float64
	Output     float64
	CacheWrite float64
	CacheRead  float64
}

// Stats is the full aggregation used by the dashboard.
type Stats struct {
	// lifetime headline (billable = input + output; cache excluded)
	Billable   int64
	Input      int64
	Output     int64
	CacheWrite int64
	CacheRead  int64
	Cost       float64
	Messages   int
	Sessions   int

	CacheHitRate float64 // cacheRead / grand total tokens

	Today PeriodStat

	Models   []ModelUsage
	Projects []ProjectUsage

	HourOfDay [24]int // message count by local hour, lifetime

	FirstDay time.Time
	LastDay  time.Time
}

// Aggregate rolls records into Stats. now sets "today" and the local time zone.
func Aggregate(records []Record, now time.Time) Stats {
	loc := now.Location()
	today := dayStart(now, loc)

	var s Stats
	sessions := map[string]bool{}
	models := map[string]*ModelUsage{}
	projects := map[string]*ProjectUsage{}

	for _, r := range records {
		bill := r.Billable()
		cost := CostOf(r)

		s.Billable += bill
		s.Input += r.Input
		s.Output += r.Output
		s.CacheWrite += r.CacheCreate
		s.CacheRead += r.CacheRead
		s.Cost += cost
		s.Messages++
		if r.SessionID != "" {
			sessions[r.SessionID] = true
		}

		ts := r.Timestamp.In(loc)
		s.HourOfDay[ts.Hour()]++
		day := dayStart(ts, loc)
		if s.FirstDay.IsZero() || day.Before(s.FirstDay) {
			s.FirstDay = day
		}
		if day.After(s.LastDay) {
			s.LastDay = day
		}
		if !day.Before(today) {
			s.Today.Billable += bill
			s.Today.Input += r.Input
			s.Today.Output += r.Output
			s.Today.Cost += cost
			s.Today.Messages++
		}

		m := models[r.Model]
		if m == nil {
			m = &ModelUsage{Model: r.Model}
			models[r.Model] = m
		}
		m.Billable += bill
		m.Cost += cost
		m.Messages++

		if r.Project != "" {
			p := projects[r.Project]
			if p == nil {
				p = &ProjectUsage{Project: r.Project}
				projects[r.Project] = p
			}
			p.Billable += bill
			p.Messages++
		}
	}

	s.Sessions = len(sessions)
	grand := s.Input + s.Output + s.CacheWrite + s.CacheRead
	if grand > 0 {
		s.CacheHitRate = float64(s.CacheRead) / float64(grand)
	}

	for _, m := range models {
		s.Models = append(s.Models, *m)
	}
	sort.Slice(s.Models, func(i, j int) bool { return s.Models[i].Billable > s.Models[j].Billable })
	for _, p := range projects {
		s.Projects = append(s.Projects, *p)
	}
	sort.Slice(s.Projects, func(i, j int) bool { return s.Projects[i].Billable > s.Projects[j].Billable })

	return s
}

// Composition returns the per-class share of the grand token total.
func (s Stats) Composition() CompositionPct {
	grand := float64(s.Input + s.Output + s.CacheWrite + s.CacheRead)
	if grand == 0 {
		return CompositionPct{}
	}
	return CompositionPct{
		Input:      float64(s.Input) / grand,
		Output:     float64(s.Output) / grand,
		CacheWrite: float64(s.CacheWrite) / grand,
		CacheRead:  float64(s.CacheRead) / grand,
	}
}
