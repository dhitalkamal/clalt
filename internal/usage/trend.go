package usage

import "time"

// DayBucket is one calendar day's aggregated usage.
type DayBucket struct {
	Day      time.Time
	Tokens   int64
	Cost     float64
	Messages int
}

// DailyTrend buckets records into the last `days` calendar days ending on now's
// day, in chronological order (oldest first). Days use now's location.
func DailyTrend(records []Record, days int, now time.Time) []DayBucket {
	if days < 1 {
		days = 1
	}
	loc := now.Location()
	today := dayStart(now, loc)
	// index day -> bucket position (0 = oldest)
	idx := map[time.Time]int{}
	buckets := make([]DayBucket, days)
	for i := 0; i < days; i++ {
		day := today.AddDate(0, 0, -(days - 1 - i))
		buckets[i] = DayBucket{Day: day}
		idx[day] = i
	}
	for _, r := range records {
		day := dayStart(r.Timestamp.In(loc), loc)
		i, ok := idx[day]
		if !ok {
			continue
		}
		buckets[i].Tokens += r.Billable()
		buckets[i].Cost += CostOf(r)
		buckets[i].Messages++
	}
	return buckets
}

func dayStart(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}
