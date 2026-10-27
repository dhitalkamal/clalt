package feed

import (
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"
)

// the child half: when FEED_READINGS_CHILD is set the test binary appends
// readings to the folder in FEED_READINGS_DIR instead of running tests.
func TestMain(m *testing.M) {
	if dir := os.Getenv("FEED_READINGS_DIR"); dir != "" {
		id := os.Getenv("FEED_READINGS_ID")
		n, _ := strconv.Atoi(os.Getenv("FEED_READINGS_N"))
		s := Store{Dir: dir, LockWait: 10 * time.Second}
		base := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
		who, _ := strconv.Atoi(id)
		for i := 0; i < n; i++ {
			// call times are at least 10 minutes apart across both processes, so the
			// skip rule never drops a line; the merged values may stay equal
			at := base.Add(time.Duration(i*20+who*10) * time.Minute)
			pct := float64(who*1000 + i)
			if err := s.MergeLimits("work", &Window{UsedPct: pct, ResetsAt: base.Add(5000 * time.Hour)}, nil, at); err != nil {
				os.Exit(2)
			}
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// two hook processes and many goroutines append at once: every line must be
// whole json, none interleaved, none lost
func TestReadingsConcurrentProcessesNeverInterleave(t *testing.T) {
	s := newStore(t)
	const per = 60
	var wg sync.WaitGroup
	for p := 1; p <= 2; p++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			cmd := exec.Command(os.Args[0])
			cmd.Env = append(os.Environ(), "FEED_READINGS_DIR="+s.Dir, "FEED_READINGS_ID="+strconv.Itoa(p), "FEED_READINGS_N="+strconv.Itoa(per))
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("child %d: %v %s", p, err, out)
			}
		}(p)
	}
	wg.Wait()
	got, err := s.Readings("work", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2*per {
		t.Fatalf("%d readings, want %d (lines lost or torn)", len(got), 2*per)
	}
	if lines := readLines(t, s, "work"); len(lines) != 2*per {
		t.Fatalf("%d lines", len(lines))
	}
}

// without the store lock, single O_APPEND writes alone keep lines whole
func TestAppendReadingLineIsOneWrite(t *testing.T) {
	s := newStore(t)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				v := float64(g*1000 + i)
				r := Reading{T: t0.Add(time.Duration(g*1000+i) * time.Second), FiveHour: &v}
				if err := s.appendLine("work", r); err != nil {
					t.Error(err)
					return
				}
			}
		}(g)
	}
	wg.Wait()
	got, err := s.Readings("work", time.Time{})
	if err != nil || len(got) != 800 {
		t.Fatalf("%d readings %v", len(got), err)
	}
}
