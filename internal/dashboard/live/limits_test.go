package live

import (
	"testing"
	"time"
)

func TestNoLimitsAreNoReading(t *testing.T) {
	if got := mapLimits(Limits{}, false, nil, epoch); got.ReadAt != (time.Time{}) || got.Five.Percent != 0 {
		t.Errorf("limits = %+v", got)
	}
}

func TestAWindowIsForecastFromItsPace(t *testing.T) {
	// 5h window that started 1h ago, 25% used: 125% at the reset, empty after 4h
	obs := epoch
	l := Limits{ObservedAt: obs, Five: Window{Percent: 25, ResetsAt: obs.Add(4 * time.Hour)}}
	got := mapLimits(l, true, nil, obs).Five
	if got.Percent != 25 || got.Projected != 125 {
		t.Fatalf("window = %+v", got)
	}
	if want := obs.Add(3 * time.Hour); !got.RunsOutAt.Equal(want) {
		t.Errorf("runs out at %v, want %v", got.RunsOutAt, want)
	}
}

func TestAWindowThatEndsAtTheResetHasNoRunOut(t *testing.T) {
	obs := epoch
	l := Limits{ObservedAt: obs, Five: Window{Percent: 10, ResetsAt: obs.Add(4 * time.Hour)}}
	got := mapLimits(l, true, nil, obs).Five
	if got.Projected != 50 || !got.RunsOutAt.IsZero() {
		t.Errorf("window = %+v", got)
	}
}

func TestNoForecastBeforeAShareOfTheWindowHasPassed(t *testing.T) {
	obs := epoch
	l := Limits{ObservedAt: obs, Five: Window{Percent: 30, ResetsAt: obs.Add(5*time.Hour - time.Minute)}}
	if got := mapLimits(l, true, nil, obs).Five; got.Projected != 0 || got.Percent != 30 {
		t.Errorf("window = %+v", got)
	}
}

func TestAWindowThatHasResetCountsAsEmpty(t *testing.T) {
	obs := epoch.Add(-6 * time.Hour)
	l := Limits{ObservedAt: obs, Five: Window{Percent: 80, ResetsAt: obs.Add(time.Hour)}, Seven: Window{Percent: 40, ResetsAt: epoch.Add(24 * time.Hour)}}
	got := mapLimits(l, true, nil, epoch)
	if got.Five.Percent != 0 || !got.Five.ResetsAt.IsZero() || got.Five.Projected != 0 {
		t.Errorf("five = %+v", got.Five)
	}
	if got.Seven.Percent != 40 || got.ReadAt != obs {
		t.Errorf("seven = %+v, read at %v", got.Seven, got.ReadAt)
	}
}

func TestTheChartTakesTheHighestReadingOfEachThreeHourSlot(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 30, 0, 0, time.UTC)
	readings := []Reading{
		{At: now.Add(-10 * time.Minute), FivePercent: 20},
		{At: now.Add(-20 * time.Minute), FivePercent: 60},
		{At: now.AddDate(0, 0, -6).Add(-13 * time.Hour), FivePercent: 99}, // before the window
	}
	got := mapLimits(Limits{ObservedAt: now}, true, readings, now)
	if len(got.Readings) != 56 || len(got.Days) != 7 {
		t.Fatalf("%d readings over %d days", len(got.Readings), len(got.Days))
	}
	if got.Readings[52] != 60 || got.Readings[51] != 0 || got.Readings[55] != 0 {
		t.Errorf("slots of today = %v", got.Readings[48:])
	}
	if got.Days[0] != "Sat" || got.Days[6] != "Fri" {
		t.Errorf("days = %v", got.Days)
	}
}

func TestNoReadingsNoChart(t *testing.T) {
	got := mapLimits(Limits{ObservedAt: epoch}, true, nil, epoch)
	if got.Readings != nil || got.Days != nil {
		t.Errorf("chart = %v %v", got.Readings, got.Days)
	}
}
