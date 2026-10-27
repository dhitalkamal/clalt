package dashboard

import (
	"testing"
	"time"
)

func TestPaceFillProjectsTheFullTimeFromTheAveragePace(t *testing.T) {
	reset := time.Date(2026, 10, 27, 15, 0, 0, 0, time.UTC)
	observed := reset.Add(-2 * time.Hour) // 3h of the 5h window have passed
	rate, at, ok := PaceFill(60, reset, observed, 5*time.Hour)
	if !ok || rate != 20 {
		t.Fatalf("rate = %v, ok = %v, want 20 per hour", rate, ok)
	}
	if want := observed.Add(2 * time.Hour); !at.Equal(want) {
		t.Errorf("full at %v, want %v", at, want)
	}
}

func TestPaceFillIsNotOkWithoutEnoughData(t *testing.T) {
	reset := time.Date(2026, 10, 27, 15, 0, 0, 0, time.UTC)
	cases := map[string]struct {
		pct      float64
		observed time.Time
	}{
		"nothing used":       {0, reset.Add(-time.Hour)},
		"first minutes":      {10, reset.Add(-5*time.Hour + 5*time.Minute)},
		"read after the end": {50, reset.Add(time.Minute)},
		"read before window": {50, reset.Add(-6 * time.Hour)},
	}
	for name, c := range cases {
		if _, _, ok := PaceFill(c.pct, reset, c.observed, 5*time.Hour); ok {
			t.Errorf("%s: ok", name)
		}
	}
}

func TestPaceFillAtOrOverFullIsFullNow(t *testing.T) {
	reset := time.Date(2026, 10, 27, 15, 0, 0, 0, time.UTC)
	observed := reset.Add(-time.Hour)
	if _, at, ok := PaceFill(100, reset, observed, 5*time.Hour); !ok || !at.Equal(observed) {
		t.Errorf("at = %v, ok = %v", at, ok)
	}
}
