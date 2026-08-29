package cli

import (
	"context"
	"sort"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/credential"
	"github.com/dhitalkamal/clalt/internal/usage"
)

// liveTTL is how long a cached live-usage response stays fresh; the endpoint
// rate-limits hard, so we poll it rarely.
const liveTTL = 5 * time.Minute

type modelStat struct {
	Model    string  `json:"model"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"cost"`
	Messages int     `json:"messages"`
}

type projectStat struct {
	Project  string `json:"project"`
	Tokens   int64  `json:"tokens"`
	Messages int    `json:"messages"`
}

type accountReport struct {
	Name      string            `json:"name"`
	Active    bool              `json:"active"`
	Plan      string            `json:"plan,omitempty"`
	Email     string            `json:"email,omitempty"`
	ExpiresAt time.Time         `json:"expiresAt,omitempty"`
	FiveHour  usage.WindowStat  `json:"fiveHour"`
	SevenDay  usage.WindowStat  `json:"sevenDay"`
	Tokens    int64             `json:"tokens"`
	Cost      float64           `json:"cost"`
	Models    []modelStat       `json:"models"`
	Projects  []projectStat     `json:"topProjects"`
	Trend     []usage.DayBucket `json:"trend"`
	Live      *usage.LiveUsage  `json:"live,omitempty"`
	LiveNote  string            `json:"liveNote,omitempty"`
}

// recordsByAccount scans local transcripts and groups usage records by the
// account active at each record's time, folding pre-first-switch usage into the
// current active account so a fresh user still sees their history.
func (a *App) recordsByAccount() (map[string][]usage.Record, error) {
	records, err := usage.ScanProjects(config.ClaudeProjectsDir())
	if err != nil {
		return nil, err
	}
	events, err := a.Store.ReadSwitchLog()
	if err != nil {
		return nil, err
	}
	usageEvents := make([]usage.SwitchEvent, 0, len(events))
	for _, e := range events {
		usageEvents = append(usageEvents, usage.SwitchEvent{Ts: e.Ts, Account: e.Account})
	}
	byAccount := usage.Attribute(records, usageEvents)
	if active, _ := a.Store.Active(); active != "" {
		if u := byAccount[usage.Unattributed]; len(u) > 0 {
			byAccount[active] = append(byAccount[active], u...)
			delete(byAccount, usage.Unattributed)
		}
	}
	return byAccount, nil
}

// buildReports assembles per-account usage from local transcripts attributed via
// the switch log, optionally augmented with live data for the filtered account.
func (a *App) buildReports(wantLive bool, filter string) ([]accountReport, error) {
	byAccount, err := a.recordsByAccount()
	if err != nil {
		return nil, err
	}
	accts, err := a.Store.List()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var reports []accountReport
	for _, ac := range accts {
		if filter != "" && ac.Name != filter {
			continue
		}
		recs := byAccount[ac.Name]
		r := buildOne(ac.Name, ac.Active, ac.Meta.Plan, ac.Meta.Email, recs, now)
		if c, _, gerr := a.Store.Get(ac.Name); gerr == nil {
			r.ExpiresAt = c.ExpiresAtTime()
			if wantLive {
				r.Live, r.LiveNote = a.liveUsage(ac.Name, c)
			}
		}
		reports = append(reports, r)
	}
	return reports, nil
}

func buildOne(name string, active bool, plan, email string, recs []usage.Record, now time.Time) accountReport {
	r := accountReport{
		Name: name, Active: active, Plan: plan, Email: email,
		FiveHour: usage.Window(recs, now, 5*time.Hour),
		SevenDay: usage.Window(recs, now, 7*24*time.Hour),
		Tokens:   0,
		Cost:     usage.TotalCost(recs),
		Trend:    usage.DailyTrend(recs, 14, now),
	}
	models := map[string]*modelStat{}
	projects := map[string]*projectStat{}
	for _, rec := range recs {
		r.Tokens += rec.Billable()
		m := models[rec.Model]
		if m == nil {
			m = &modelStat{Model: rec.Model}
			models[rec.Model] = m
		}
		m.Tokens += rec.Billable()
		m.Cost += usage.CostOf(rec)
		m.Messages++
		if rec.Project != "" {
			p := projects[rec.Project]
			if p == nil {
				p = &projectStat{Project: rec.Project}
				projects[rec.Project] = p
			}
			p.Tokens += rec.Billable()
			p.Messages++
		}
	}
	for _, m := range models {
		r.Models = append(r.Models, *m)
	}
	sort.Slice(r.Models, func(i, j int) bool { return r.Models[i].Tokens > r.Models[j].Tokens })
	for _, p := range projects {
		r.Projects = append(r.Projects, *p)
	}
	sort.Slice(r.Projects, func(i, j int) bool { return r.Projects[i].Tokens > r.Projects[j].Tokens })
	if len(r.Projects) > 5 {
		r.Projects = r.Projects[:5] // top 5
	}
	return r
}

// liveUsage returns cached live data when fresh, otherwise fetches; on a rate
// limit it falls back to any stale cache and notes that.
func (a *App) liveUsage(name string, cred credential.Credential) (*usage.LiveUsage, string) {
	now := time.Now()
	if e, ok := a.Cache.Load(name); ok && e.Fresh(liveTTL, now) {
		d := e.Data
		return &d, "cached"
	}
	if a.ApiBase == "" {
		return nil, "live disabled"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lu, err := usage.FetchLive(ctx, a.OAuth.HTTP, a.ApiBase, cred.AccessToken)
	if err != nil {
		if e, ok := a.Cache.Load(name); ok {
			d := e.Data
			if usage.IsRateLimited(err) {
				return &d, "rate limited; showing stale cache"
			}
			return &d, "live fetch failed; showing stale cache"
		}
		if usage.IsRateLimited(err) {
			return nil, "rate limited; no cached data"
		}
		return nil, "live fetch failed: " + err.Error()
	}
	_ = a.Cache.Save(name, lu, now)
	return &lu, "live"
}
