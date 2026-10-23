package live

import (
	"time"

	"github.com/dhitalkamal/clalt/internal/dashboard"
)

const (
	fiveHours = 5 * time.Hour
	sevenDays = 7 * 24 * time.Hour
	// no forecast until this share of the window has passed, or the first
	// minutes would promise a run out that is not coming
	minElapsed = 0.05

	slotsPerDay = 8
	chartDays   = 7
)

// mapLimits turns what the feed reported into the dashboard's limits. a window
// that has reset since the reading counts as empty, and the reading's age says
// how old the rest is. readings are the 5h history for the chart.
func mapLimits(l Limits, ok bool, readings []Reading, now time.Time) dashboard.Limits {
	if !ok {
		return dashboard.Limits{}
	}
	out := dashboard.Limits{
		Five:   mapWindow(l.Five, l.ObservedAt, fiveHours, now),
		Seven:  mapWindow(l.Seven, l.ObservedAt, sevenDays, now),
		ReadAt: l.ObservedAt,
	}
	out.Readings, out.Days = chart(readings, now)
	return out
}

func mapWindow(w Window, observed time.Time, length time.Duration, now time.Time) dashboard.Window {
	if w.Unknown {
		return dashboard.Window{Unknown: true}
	}
	if !w.ResetsAt.IsZero() && !w.ResetsAt.After(now) {
		return dashboard.Window{}
	}
	out := dashboard.Window{Percent: clamp(w.Percent), ResetsAt: w.ResetsAt}
	if w.ResetsAt.IsZero() || out.Percent <= 0 {
		return out
	}
	elapsed := observed.Sub(w.ResetsAt.Add(-length))
	if float64(elapsed) < minElapsed*float64(length) || elapsed > length {
		return out
	}
	pace := float64(out.Percent) / float64(elapsed)
	out.Projected = min(int(pace*float64(length)+0.5), 999)
	if out.Percent >= 100 {
		out.RunsOutAt = observed
	} else if full := observed.Add(time.Duration(float64(100-out.Percent) / pace)); full.Before(w.ResetsAt) {
		out.RunsOutAt = full
	}
	return out
}

func clamp(p int) int { return max(0, min(p, 100)) }

// chart buckets the readings into 3 hour slots over the last 7 days, each with
// the highest reading in it, and names the days. no readings, no chart.
func chart(readings []Reading, now time.Time) ([]float64, []string) {
	if len(readings) == 0 {
		return nil, nil
	}
	slot := 24 * time.Hour / slotsPerDay
	today := startOfDay(now)
	first := today.AddDate(0, 0, -(chartDays - 1))
	vals := make([]float64, chartDays*slotsPerDay)
	for _, r := range readings {
		at := r.At.In(now.Location())
		if at.Before(first) || at.After(now) {
			continue
		}
		i := int(at.Sub(first) / slot)
		if i >= len(vals) {
			i = len(vals) - 1
		}
		vals[i] = max(vals[i], float64(clamp(r.FivePercent)))
	}
	days := make([]string, chartDays)
	for i := range days {
		days[i] = first.AddDate(0, 0, i).Format("Mon")
	}
	return vals, days
}
