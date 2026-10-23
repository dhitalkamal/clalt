package feed

import (
	"sync"
	"testing"
	"time"
)

// sessions of one account report at the same time: nothing may be lost or torn
func TestParallelReportsKeepTheHighestUseAndEveryUpdate(t *testing.T) {
	s := newStore(t)
	s.LockWait = 5 * time.Second
	resets := t0.Add(time.Hour)
	var wg sync.WaitGroup
	errs := make(chan error, 60)
	for i := 1; i <= 30; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			errs <- s.MergeLimits("work", &Window{UsedPct: float64(i), ResetsAt: resets}, nil, t0)
		}(i)
		go func(i int) {
			defer wg.Done()
			errs <- s.UpdateSession(h1, "work", t0, func(x *SessionState) bool { x.Cwd = "/c"; return true })
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if l, ok := s.Limits("work"); !ok || l.FiveHour.UsedPct != 30 {
		t.Errorf("%+v %v", l, ok)
	}
	if st, ok := s.Session(h1); !ok || st.Cwd != "/c" {
		t.Errorf("%+v %v", st, ok)
	}
}
