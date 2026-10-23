package cli

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/usage"
)

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

// accountReport is the usage of one account. name is its label, id the folder
// it is kept in; usage nobody can be blamed for is reported as unattributed,
// under that name and id.
type accountReport struct {
	ID       string            `json:"id,omitempty"`
	Name     string            `json:"name"`
	Plan     string            `json:"plan,omitempty"`
	Email    string            `json:"email,omitempty"`
	FiveHour usage.WindowStat  `json:"fiveHour"`
	SevenDay usage.WindowStat  `json:"sevenDay"`
	Tokens   int64             `json:"tokens"`
	Cost     float64           `json:"cost"`
	Models   []modelStat       `json:"models"`
	Projects []projectStat     `json:"topProjects"`
	Trend    []usage.DayBucket `json:"trend"`
}

// recordsByAccount scans local transcripts and groups usage records by the
// account id that was active at each record's time, per the switch log an old
// clalt kept. what the log cannot place stays under usage.Unattributed, and so
// does everything from the first sign-in of the folder design on, unless a
// session of clalt run saw its claude session id, which says whose it is. records
// are deduped by message id and request id, so a reply Claude Code wrote as
// several transcript lines counts once. the transcripts are the ones of the
// shared home, and when they cannot be read that is an error: the numbers would
// be missing, not zero.
func (a *App) recordsByAccount() (map[string][]usage.Record, error) {
	projects := config.ClaudeProjectsDir()
	if projects == "" {
		return nil, errors.New(noHomeKnown)
	}
	records, err := usage.ScanRoots([]string{projects})
	if err != nil {
		return nil, fmt.Errorf("read the transcripts in %s: %w", projects, err)
	}
	events, err := a.Store.ReadSwitchLog()
	if err != nil {
		return nil, err
	}
	usageEvents := make([]usage.SwitchEvent, 0, len(events))
	for _, e := range events {
		usageEvents = append(usageEvents, usage.SwitchEvent{Ts: e.Ts, Account: e.Account})
	}
	// the log stopped growing when the first account signed in as a folder, and
	// what was written after that has no account to be charged to
	cutover, err := a.Store.ThinSince()
	if err != nil {
		return nil, err
	}
	sessions, err := usage.ReadSessionAccounts(config.SessionAccountsPath())
	if err != nil {
		return nil, err
	}
	return usage.AttributeSessions(records, usageEvents, sessions, cutover), nil
}

// buildReports assembles per-account usage from local transcripts. with a label
// it reports that one account; without, every visible account, then whatever
// usage no account could be given.
func (a *App) buildReports(label string) ([]accountReport, error) {
	byAccount, err := a.recordsByAccount()
	if err != nil {
		return nil, err
	}
	accts, err := a.visibleAccounts()
	if err != nil {
		return nil, err
	}
	if label != "" {
		acc, err := a.findVisible(label)
		if err != nil {
			return nil, err
		}
		accts = []account.Account{acc}
	}
	now := a.clock()
	var reports []accountReport
	for _, acc := range accts {
		r := buildOne(shown(acc), byAccount[acc.Name], now)
		r.ID, r.Plan, r.Email = acc.Name, clean(acc.Meta.Plan), clean(acc.Meta.Email)
		reports = append(reports, r)
	}
	if recs := byAccount[usage.Unattributed]; label == "" && len(recs) > 0 {
		rest := buildOne(usage.Unattributed, recs, now)
		rest.ID = usage.Unattributed
		reports = append(reports, rest)
	}
	return reports, nil
}

func buildOne(name string, recs []usage.Record, now time.Time) accountReport {
	r := accountReport{
		Name:     name,
		FiveHour: usage.Window(recs, now, 5*time.Hour),
		SevenDay: usage.Window(recs, now, 7*24*time.Hour),
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
