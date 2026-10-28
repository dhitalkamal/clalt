package feed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func readLines(t *testing.T, s Store, account string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(s.Dir, "readings", account+".jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

func TestMergeLimitsAppendsOneCompactLine(t *testing.T) {
	s := newStore(t)
	r5, r7 := t0.Add(2*time.Hour), t0.Add(48*time.Hour)
	if err := s.MergeLimits("work", win(41.5, r5), win(12, r7), t0); err != nil {
		t.Fatal(err)
	}
	lines := readLines(t, s, "work")
	if len(lines) != 1 {
		t.Fatalf("lines: %q", lines)
	}
	want := `{"t":"2026-10-23T12:00:00Z","fiveHour":41.5,"fiveHourResetsAt":"2026-10-23T14:00:00Z","sevenDay":12,"sevenDayResetsAt":"2026-10-25T12:00:00Z"}`
	if lines[0] != want {
		t.Errorf("got  %s\nwant %s", lines[0], want)
	}
}

func TestReadingLineHasNullForAMissingWindow(t *testing.T) {
	s := newStore(t)
	if err := s.MergeLimits("work", win(10, t0.Add(time.Hour)), nil, t0); err != nil {
		t.Fatal(err)
	}
	l := readLines(t, s, "work")[0]
	if !strings.Contains(l, `"sevenDay":null,"sevenDayResetsAt":null`) {
		t.Errorf("%s", l)
	}
}

func TestReadingsSkipUnchangedWithinFiveMinutes(t *testing.T) {
	s := newStore(t)
	r := t0.Add(2 * time.Hour)
	steps := []struct {
		min  int
		pct  float64
		want int
	}{
		{0, 10, 1},
		{1, 10, 1},  // same value, 1 minute after the line: skipped
		{4, 10, 1},  // same value, 4 minutes: skipped
		{5, 11, 2},  // changed: written
		{6, 11, 2},  // same again, 1 minute after the last line: skipped
		{13, 11, 3}, // unchanged but 8 minutes after the last line
	}
	for i, st := range steps {
		if err := s.MergeLimits("work", win(st.pct, r), nil, t0.Add(time.Duration(st.min)*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if n := len(readLines(t, s, "work")); n != st.want {
			t.Fatalf("step %d: %d lines, want %d", i, n, st.want)
		}
	}
}

func TestReadingsNothingObservedWritesNoFile(t *testing.T) {
	s := newStore(t)
	if err := s.MergeLimits("work", nil, nil, t0); err != nil {
		t.Fatal(err)
	}
	if lines := readLines(t, s, "work"); lines != nil {
		t.Errorf("%q", lines)
	}
}

func TestReadingsFileModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no unix modes")
	}
	s := newStore(t)
	if err := s.MergeLimits("work", win(10, t0.Add(time.Hour)), nil, t0); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(s.Dir, "readings")
	di, err := os.Stat(dir)
	if err != nil || di.Mode().Perm() != 0o700 {
		t.Errorf("dir %v %v", di, err)
	}
	fi, err := os.Stat(filepath.Join(dir, "work.jsonl"))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("file %v %v", fi, err)
	}
}

func TestReadingsRotateDropsLinesOlderThanNinetyDays(t *testing.T) {
	s := newStore(t)
	path := filepath.Join(s.Dir, "readings", "work.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, age := range []int{120, 100, 91, 89, 30, 1} {
		ts := t0.AddDate(0, 0, -age)
		line, _ := json.Marshal(Reading{T: ts, FiveHour: ptr(float64(age))})
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.MergeLimits("work", win(5, t0.Add(time.Hour)), nil, t0); err != nil {
		t.Fatal(err)
	}
	got, err := s.Readings("work", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	var ages []float64
	for _, r := range got {
		if r.FiveHour != nil {
			ages = append(ages, *r.FiveHour)
		}
	}
	// 89, 30, 1 stay, then the new one (5); 91 is past the 90 day cut
	if len(got) != 4 || ages[0] != 89 || ages[3] != 5 {
		t.Errorf("%v %+v", ages, got)
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
			t.Errorf("mode after rotate %v", fi.Mode())
		}
	}
}

func TestReadingsNoRotationWhileTheOldestIsWithinADayOfTheCut(t *testing.T) {
	s := newStore(t)
	path := filepath.Join(s.Dir, "readings", "work.jsonl")
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	line, _ := json.Marshal(Reading{T: t0.AddDate(0, 0, -90).Add(-time.Hour), FiveHour: ptr(1)})
	if err := os.WriteFile(path, append(line, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.MergeLimits("work", win(5, t0.Add(time.Hour)), nil, t0); err != nil {
		t.Fatal(err)
	}
	if n := len(readLines(t, s, "work")); n != 2 {
		t.Errorf("rotated too early: %d lines", n)
	}
}

func TestReadingsSinceFiltersAndSkipsCorruptLines(t *testing.T) {
	s := newStore(t)
	path := filepath.Join(s.Dir, "readings", "work.jsonl")
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	l1, _ := json.Marshal(Reading{T: t0, FiveHour: ptr(1)})
	l2, _ := json.Marshal(Reading{T: t0.Add(time.Hour), FiveHour: ptr(2)})
	body := string(l1) + "\nnot json\n" + string(l2) + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := s.Readings("work", t0.Add(time.Minute))
	if err != nil || len(got) != 1 || *got[0].FiveHour != 2 {
		t.Errorf("%+v %v", got, err)
	}
	if none, err := s.Readings("nobody", time.Time{}); err != nil || len(none) != 0 {
		t.Errorf("%+v %v", none, err)
	}
}

func TestReadingsRejectABadAccountID(t *testing.T) {
	s := newStore(t)
	if _, err := s.Readings("../x", time.Time{}); err == nil {
		t.Error("accepted a path as an account")
	}
}

// a torn tail (a crash mid-write) must not stop later appends
func TestReadingsAppendAfterATornLastLine(t *testing.T) {
	s := newStore(t)
	path := filepath.Join(s.Dir, "readings", "work.jsonl")
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	if err := os.WriteFile(path, []byte(`{"t":"2026-10-2`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.MergeLimits("work", win(5, t0.Add(time.Hour)), nil, t0); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Readings("work", time.Time{})
	if len(got) != 1 || *got[0].FiveHour != 5 {
		t.Errorf("%+v", got)
	}
}

func ptr(v float64) *float64 { return &v }
