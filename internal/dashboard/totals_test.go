package dashboard

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 0.005 }

func TestTotalsComeFromTheDays(t *testing.T) {
	a := testAccount()
	if got := a.today(); !near(got.cost, 2.14) || got.tokens != 528_580 {
		t.Errorf("today = %+v, want cost 2.14 and 528580 tokens", got)
	}
	if got := a.week(); !near(got.cost, 18.40) {
		t.Errorf("week cost = %v, want 18.40", got.cost)
	}
	if got := a.monthCost(); !near(got, 71.20) {
		t.Errorf("30 day cost = %v, want 71.20", got)
	}
	want := []float64{3.37, 4.05, 1.40, 0.95, 2.87, 3.62, 2.14}
	got := a.weekCosts()
	if len(got) != 7 {
		t.Fatalf("weekCosts has %d days", len(got))
	}
	for i := range want {
		if !near(got[i], want[i]) {
			t.Errorf("weekCosts[%d] = %v, want %v (oldest first)", i, got[i], want[i])
		}
	}
	series := a.monthCosts()
	if len(series) != 30 || !near(series[29], 2.14) || !near(series[0], 1.80) {
		t.Errorf("monthCosts should run oldest first over 30 days, got %d values from %v to %v", len(series), series[0], series[len(series)-1])
	}
}

func TestTotalsWithFewOrNoDays(t *testing.T) {
	var a Account
	if a.today() != (usageTotal{}) || a.week() != (usageTotal{}) || a.monthCost() != 0 {
		t.Error("no days means zero totals")
	}
	if len(a.weekCosts()) != 0 || len(a.monthCosts()) != 0 {
		t.Error("no days means no series")
	}
	a.History.Days = []Day{{Cost: 1.5, Input: 100, Output: 50}, {Cost: 2.5, Input: 10}}
	if got := a.week(); !near(got.cost, 4) || got.tokens != 160 {
		t.Errorf("a short history sums what it has, got %+v", got)
	}
	if got := a.weekCosts(); len(got) != 2 || got[0] != 2.5 {
		t.Errorf("weekCosts of two days = %v", got)
	}
}

func TestLoginLevel(t *testing.T) {
	cases := []struct {
		days int
		want Level
	}{{30, LevelOK}, {23, LevelOK}, {4, LevelOK}, {3, LevelWarn}, {2, LevelWarn}, {1, LevelBad}, {0, LevelBad}, {-2, LevelBad}}
	for _, c := range cases {
		if got := loginLevel(c.days); got != c.want {
			t.Errorf("loginLevel(%d) = %v, want %v", c.days, got, c.want)
		}
	}
}
