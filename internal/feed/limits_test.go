package feed

import (
	"testing"
	"time"
)

func win(pct float64, resets time.Time) *Window { return &Window{UsedPct: pct, ResetsAt: resets} }

func TestMergeLimitsRules(t *testing.T) {
	r1, r2 := t0.Add(2*time.Hour), t0.Add(4*time.Hour)
	past := t0.Add(-time.Minute)
	cases := []struct {
		name     string
		old, obs *Window
		want     *Window
	}{
		{"first observation", nil, win(20, r1), win(20, r1)},
		{"higher use in the same window wins", win(20, r1), win(35, r1), win(35, r1)},
		{"an old session cannot lower it", win(60, r1), win(20, r1), win(60, r1)},
		{"a later window wins", win(60, r1), win(5, r2), win(5, r2)},
		{"an earlier window is stale", win(5, r2), win(60, r1), win(5, r2)},
		{"an expired observation is ignored", win(40, r1), win(90, past), win(40, r1)},
		{"an expired old window is dropped", win(40, past), nil, nil},
		{"a reset time a few seconds off is the same window", win(60, r1), win(20, r1.Add(30*time.Second)), win(60, r1)},
		{"nothing observed keeps the old", win(40, r1), nil, win(40, r1)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mergeWindow(c.old, c.obs, t0)
			if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestMergeLimitsStoresAndReadsPerAccount(t *testing.T) {
	s := newStore(t)
	r1 := t0.Add(time.Hour)
	if err := s.MergeLimits("work", win(10, r1), win(50, t0.Add(24*time.Hour)), t0); err != nil {
		t.Fatal(err)
	}
	if err := s.MergeLimits("work", win(30, r1), nil, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, ok := s.Limits("work")
	if !ok || got.FiveHour.UsedPct != 30 || got.SevenDay.UsedPct != 50 || !got.ObservedAt.Equal(t0.Add(time.Minute)) || got.Account != "work" {
		t.Errorf("%+v %v", got, ok)
	}
	if _, ok := s.Limits("other"); ok {
		t.Error("limits for an account nobody reported")
	}
	all, err := s.AllLimits()
	if err != nil || len(all) != 1 {
		t.Errorf("%v %v", all, err)
	}
}

func TestMergeLimitsRefusesAnAccountThatIsNotAFolderName(t *testing.T) {
	s := newStore(t)
	if err := s.MergeLimits("../x", win(1, t0.Add(time.Hour)), nil, t0); err == nil {
		t.Error("written")
	}
	if _, ok := s.Limits("../x"); ok {
		t.Error("read")
	}
}

func TestMergeLimitsWithNothingObservedWritesNothing(t *testing.T) {
	s := newStore(t)
	if err := s.MergeLimits("work", nil, nil, t0); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Limits("work"); ok {
		t.Error("a file was written")
	}
}
