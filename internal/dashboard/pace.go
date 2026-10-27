package dashboard

import (
	"fmt"
	"time"
)

// the forecast in a Window is numbers and times. these turn it into the words
// the overview and the limits view share.

const (
	imminent = 15 * time.Minute // a run out this close is red
	soon     = time.Hour        // a run out closer than this is told in minutes
)

func hasForecast(w Window) bool { return w.Projected > 0 || !w.RunsOutAt.IsZero() }

func runsOut(w Window) bool { return !w.RunsOutAt.IsZero() || w.Projected >= 100 }

// paceLevel is how worried the forecast should look
func paceLevel(w Window, now time.Time) Level {
	switch {
	case !w.RunsOutAt.IsZero() && w.RunsOutAt.Sub(now) < imminent:
		return LevelBad
	case runsOut(w), w.Projected >= 90:
		return LevelWarn
	}
	return LevelOK
}

// paceShort is the line under a bar on the overview, empty without a forecast
func paceShort(w Window, weekly bool, now time.Time) string {
	switch {
	case !hasForecast(w):
		return ""
	case !w.RunsOutAt.IsZero():
		return "at this pace it runs out " + runOutWhen(w.RunsOutAt, now)
	case w.Projected >= 100:
		return "at this pace it runs out before it resets"
	case weekly:
		return fmt.Sprintf("on pace to finish the week at %d%%", w.Projected)
	}
	return fmt.Sprintf("on pace to end this window at %d%%", w.Projected)
}

// paceLong is the sentence on the limits view, empty without a forecast
func paceLong(w Window, weekly bool, now time.Time) string {
	switch {
	case !hasForecast(w):
		return ""
	case !w.RunsOutAt.IsZero():
		return longRunOut(w, now)
	case w.Projected >= 100:
		return "runs out before it resets"
	case weekly:
		return fmt.Sprintf("%d%% now, on pace for %d%% by %s", w.Percent, w.Projected, resetClock(w, true, now))
	}
	return fmt.Sprintf("on pace to end this window at %d%%, %s", w.Projected, roomWord(w.Projected))
}

func longRunOut(w Window, now time.Time) string {
	s := "runs out " + runOutWhen(w.RunsOutAt, now)
	gap := w.ResetsAt.Sub(w.RunsOutAt)
	if gap <= 0 {
		return s
	}
	// a run out told in minutes is exact enough that the gap needs no hedge
	hedge := "about "
	if w.RunsOutAt.Sub(now) < soon {
		hedge = ""
	}
	return s + ", " + hedge + roughSpan(gap) + " before it resets"
}

func roomWord(projected int) string {
	switch {
	case projected < 25:
		return "no risk of running out"
	case projected < 70:
		return "plenty of room"
	case projected < 90:
		return "some room left"
	}
	return "close to the limit"
}

// runOutWhen says a run out in minutes when it is near and by the clock when
// it is not, with the weekday once it is a day or more away
func runOutWhen(at, now time.Time) string {
	d := at.Sub(now)
	switch minutes := int(d / time.Minute); {
	case d <= 0:
		return "now"
	case d < soon && minutes < 1:
		return "in under a minute"
	case d < soon && minutes == 1:
		return "in about 1 minute"
	case d < soon:
		return fmt.Sprintf("in about %d minutes", minutes)
	case d < 24*time.Hour:
		return "around " + at.In(now.Location()).Format("15:04")
	}
	return "around " + at.In(now.Location()).Format("Mon 15:04")
}

// roughSpan rounds a gap to minutes, hours or days, whichever reads best
func roughSpan(d time.Duration) string {
	switch {
	case d < 90*time.Minute:
		return plural(int(d.Round(time.Minute)/time.Minute), "minute")
	case d < 36*time.Hour:
		return plural(int(d.Round(time.Hour)/time.Hour), "hour")
	}
	return plural(int(d.Round(24*time.Hour)/(24*time.Hour)), "day")
}

// countdownText is the time left to a reset: "in 1h52m" for the 5h window,
// "1d 15h" for the week
func countdownText(d time.Duration, weekly bool) string {
	d = max(d, 0)
	if !weekly {
		return fmt.Sprintf("in %dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd %dh", int(d/(24*time.Hour)), int(d.Hours())%24)
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}

// resetClock is the reset time in the clock's zone, with the weekday for the week
func resetClock(w Window, weekly bool, now time.Time) string {
	t := w.ResetsAt.In(now.Location())
	if weekly {
		return t.Format("Mon 15:04")
	}
	return t.Format("15:04")
}
