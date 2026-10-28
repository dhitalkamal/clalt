package summary

import (
	"sort"
	"time"

	"github.com/dhitalkamal/clalt/internal/usage"
)

const (
	fiveHours = 5 * time.Hour
	sevenDays = 7 * 24 * time.Hour
)

// buildHistory reads the records of the last days days plus the week before
// them, which the 7d peak of the first day needs, and totals each day.
func (s *Service) buildHistory(today time.Time, days int) (History, error) {
	first := today.AddDate(0, 0, -(days - 1))
	end := today.AddDate(0, 0, 1)
	sc, truncated := readAll(candidates(s.cfg.Roots, first.Add(-sevenDays)), s.cfg.MaxFiles, s.cfg.MaxBytes, window{})
	order, err := s.accounts()
	if err != nil {
		return History{}, err
	}
	by, err := s.attribute(usage.Dedup(sc.records))
	if err != nil {
		return History{}, err
	}
	h := History{V: Version, Days: days, From: first.Format("2006-01-02"), To: today.Format("2006-01-02"),
		Truncated: truncated, Accounts: []HistoryAccount{}}
	have := keys(by)
	// accounts that are listed show a flat line even with no usage
	for _, a := range order {
		if _, ok := by[a.ID]; !ok {
			have = append(have, a.ID)
		}
	}
	for _, id := range accountOrder(order, have) {
		acct := HistoryAccount{ID: id, Label: labelOf(order, id), Days: s.accountDays(by[id], first, end, days)}
		s.historyPercents(acct.Days, id, first, end)
		if id != usage.Unattributed || hasUsage(acct.Days) {
			h.Accounts = append(h.Accounts, acct)
		}
	}
	s.todayReading(&h, today)
	return h, nil
}

func hasUsage(days []HistoryDay) bool {
	for _, d := range days {
		if d.Messages > 0 {
			return true
		}
	}
	return false
}

// accountDays totals the records of one account per day and finds the highest
// 5h and 7d sums that ended on each day.
func (s *Service) accountDays(recs []usage.Record, first, end time.Time, days int) []HistoryDay {
	sort.SliceStable(recs, func(i, j int) bool { return recs[i].Timestamp.Before(recs[j].Timestamp) })
	out := make([]HistoryDay, days)
	for i := range out {
		out[i].Day = first.AddDate(0, 0, i).Format("2006-01-02")
	}
	prefix := make([]int64, len(recs)+1)
	for i, r := range recs {
		prefix[i+1] = prefix[i] + r.Billable()
	}
	lo5, lo7 := 0, 0
	for i, r := range recs {
		t := r.Timestamp
		for recs[lo5].Timestamp.Compare(t.Add(-fiveHours)) <= 0 {
			lo5++
		}
		for recs[lo7].Timestamp.Compare(t.Add(-sevenDays)) <= 0 {
			lo7++
		}
		if t.Before(first) || !t.Before(end) {
			continue
		}
		d := &out[dayIndex(t.In(s.cfg.Loc), first, s.cfg.Loc)]
		d.Tokens += r.Billable()
		d.Cost += usage.CostOf(r)
		d.Messages++
		d.Peak5hTokens = max(d.Peak5hTokens, prefix[i+1]-prefix[lo5])
		d.Peak7dTokens = max(d.Peak7dTokens, prefix[i+1]-prefix[lo7])
	}
	for i := range out {
		out[i].Cost = round4(out[i].Cost)
	}
	return out
}

// dayIndex counts calendar days from first, so a day of 23 or 25 hours is one.
func dayIndex(t, first time.Time, loc *time.Location) int {
	y, m, d := t.Date()
	fy, fm, fd := first.Date()
	a := time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
	b := time.Date(fy, fm, fd, 12, 0, 0, 0, time.UTC)
	return int(a.Sub(b) / (24 * time.Hour))
}

// todayReading puts the live percent on today's day when the readings log has
// nothing for it, say before the log began.
func (s *Service) todayReading(h *History, today time.Time) {
	if s.cfg.Limits == nil {
		return
	}
	for i := range h.Accounts {
		five, seven := s.cfg.Limits(h.Accounts[i].ID)
		d := &h.Accounts[i].Days[len(h.Accounts[i].Days)-1]
		if d.Peak5hPct == nil {
			d.Peak5hPct = five
		}
		if d.Peak7dPct == nil {
			d.Peak7dPct = seven
		}
	}
}
