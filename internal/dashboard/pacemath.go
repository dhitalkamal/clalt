package dashboard

import "time"

// MinElapsed is the share of a window that must have passed before a pace is
// believed: the first minutes would promise a run out that is not coming.
const MinElapsed = 0.05

// PaceFill is how fast a window of the given length fills at its average pace so
// far, and when it reaches 100 percent. percent was read at observed and the
// window ends at resetsAt. ok is false when there is no pace to speak of: nothing
// used yet, or a reading from outside the window, or too early in it. fullAt may
// be after resetsAt, the caller decides what that means.
func PaceFill(percent float64, resetsAt, observed time.Time, length time.Duration) (perHour float64, fullAt time.Time, ok bool) {
	if resetsAt.IsZero() || percent <= 0 {
		return 0, time.Time{}, false
	}
	elapsed := observed.Sub(resetsAt.Add(-length))
	if float64(elapsed) < MinElapsed*float64(length) || elapsed > length {
		return 0, time.Time{}, false
	}
	perNs := percent / float64(elapsed)
	perHour = perNs * float64(time.Hour)
	if percent >= 100 {
		return perHour, observed, true
	}
	return perHour, observed.Add(time.Duration((100 - percent) / perNs)), true
}
