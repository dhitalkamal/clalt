package cli

import (
	"sort"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/office"
	"github.com/dhitalkamal/clalt/internal/office/summary"
	"github.com/dhitalkamal/clalt/internal/usage"
)

// officeSummary is the daily summary and history of this machine. it reads
// the transcripts of the shared home and the usage ledger, and writes
// nothing: the ledger lines that name plain claude's account are not added
// here, because only part of the records is read and a session that began
// earlier would be mistaken for a new one.
func (a *App) officeSummary() *summary.Service {
	real := a.officeReal()
	cfg := summary.Config{
		Accounts: func() ([]summary.Account, error) {
			list, err := officeAccounts{app: a}.Accounts()
			if err != nil {
				return nil, err
			}
			out := make([]summary.Account, len(list))
			for i, acc := range list {
				out[i] = summary.Account{ID: acc.ID, Label: acc.Label}
			}
			sort.SliceStable(out, func(i, j int) bool {
				if out[i].Label != out[j].Label {
					return out[i].Label < out[j].Label
				}
				return out[i].ID < out[j].ID
			})
			return out, nil
		},
		Attribute: a.attributeRecords,
		// the readings log of the feed gives every day its percent; the live
		// reading only stands in for today before the log has a line
		Readings: func(id string, since time.Time) []summary.Reading {
			rs, err := feed.Store{Dir: config.FeedDir()}.Readings(id, since)
			if err != nil {
				return nil
			}
			out := make([]summary.Reading, len(rs))
			for i, r := range rs {
				out[i] = summary.Reading{T: r.T, FiveHour: r.FiveHour, SevenDay: r.SevenDay}
			}
			return out
		},
		Limits: func(id string) (*float64, *float64) {
			l, ok := real.Limits(id)
			if !ok {
				return nil, nil
			}
			pct := func(w *office.Window) *float64 {
				if w == nil || !w.ResetsAt.After(a.clock()) {
					return nil
				}
				v := w.UsedPct
				return &v
			}
			return pct(l.FiveHour), pct(l.SevenDay)
		},
		Now: a.clock,
	}
	if dir := config.ClaudeProjectsDir(); dir != "" {
		cfg.Roots = []string{dir}
	}
	return summary.New(cfg)
}

// attributeRecords places records on accounts the way recordsByAccount does,
// from the switch log and the session ledger as they are.
func (a *App) attributeRecords(records []usage.Record) (map[string][]usage.Record, error) {
	events, err := a.Store.ReadSwitchLog()
	if err != nil {
		return nil, err
	}
	switches := make([]usage.SwitchEvent, 0, len(events))
	for _, e := range events {
		switches = append(switches, usage.SwitchEvent{Ts: e.Ts, Account: e.Account})
	}
	cutover, err := a.Store.ThinSince()
	if err != nil {
		return nil, err
	}
	sessions, err := usage.ReadSessionAccounts(config.SessionAccountsPath())
	if err != nil {
		return nil, err
	}
	return usage.AttributeSessions(records, switches, sessions, cutover), nil
}
