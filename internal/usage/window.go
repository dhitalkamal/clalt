package usage

import "time"

// WindowStat summarizes usage inside a rolling time window.
type WindowStat struct {
	Duration time.Duration
	Tokens   int64
	Messages int
	FirstAt  time.Time // earliest record inside the window
	ResetsAt time.Time // FirstAt + Duration (local estimate of the reset)
}

// Window sums records whose timestamp falls within [now-dur, now]. ResetsAt is a
// local approximation: Claude resets the window relative to the first message in
// it, so we anchor to the earliest in-window record. Authoritative numbers need
// the --live endpoint; this is the documented local estimate.
func Window(records []Record, now time.Time, dur time.Duration) WindowStat {
	cutoff := now.Add(-dur)
	ws := WindowStat{Duration: dur}
	for _, r := range records {
		if r.Timestamp.Before(cutoff) || r.Timestamp.After(now) {
			continue
		}
		ws.Tokens += r.Billable()
		ws.Messages++
		if ws.FirstAt.IsZero() || r.Timestamp.Before(ws.FirstAt) {
			ws.FirstAt = r.Timestamp
		}
	}
	if !ws.FirstAt.IsZero() {
		ws.ResetsAt = ws.FirstAt.Add(dur)
	}
	return ws
}
