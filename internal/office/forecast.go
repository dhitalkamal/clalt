package office

import (
	"math"
	"time"

	"github.com/dhitalkamal/clalt/internal/dashboard"
)

const (
	fiveHours = 5 * time.Hour
	// a reading older than this says too little about the pace right now
	maxReadAge = 30 * time.Minute
)

// ForecastOut says when an account reaches its 5h limit at the pace of the
// window so far. FiveHourAt is null when there is no live session, no recent
// reading, too little data, or the window resets first.
type ForecastOut struct {
	FiveHourAt     *time.Time `json:"fiveHourAt"`
	PacePctPerHour float64    `json:"pacePctPerHour"`
}

// forecastOf uses the dashboard's pace math. live says the account has a
// session that is not over: with none the pace is not going to continue.
func forecastOf(l Limits, live bool, now time.Time) ForecastOut {
	w := l.FiveHour
	if !live || w == nil || l.ReadAt.IsZero() || now.Sub(l.ReadAt) > maxReadAge || !w.ResetsAt.After(now) {
		return ForecastOut{}
	}
	perHour, full, ok := dashboard.PaceFill(w.UsedPct, w.ResetsAt, l.ReadAt, fiveHours)
	if !ok || !full.Before(w.ResetsAt) {
		return ForecastOut{}
	}
	if full.Before(now) {
		full = now
	}
	return ForecastOut{FiveHourAt: &full, PacePctPerHour: math.Round(perHour*10) / 10}
}
