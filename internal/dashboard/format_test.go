package dashboard

import (
	"testing"
	"time"
)

func TestMoneyAndApprox(t *testing.T) {
	cases := []struct {
		v           float64
		money, estd string
	}{
		{0, "$0.00", "~$0.00"},
		{2.14, "$2.14", "~$2.14"},
		{18.4, "$18.40", "~$18.40"},
		{0.005, "$0.01", "~$0.01"},
		{1234.5, "$1234.50", "~$1234.50"},
	}
	for _, c := range cases {
		if got := money(c.v); got != c.money {
			t.Errorf("money(%v) = %q, want %q", c.v, got, c.money)
		}
		if got := approx(c.v); got != c.estd {
			t.Errorf("approx(%v) = %q, want %q", c.v, got, c.estd)
		}
	}
}

func TestPlural(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{{0, "0 sessions"}, {1, "1 session"}, {2, "2 sessions"}, {23, "23 sessions"}}
	for _, c := range cases {
		if got := plural(c.n, "session"); got != c.want {
			t.Errorf("plural(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestAgoText(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "just now"},
		{900 * time.Millisecond, "just now"},
		{time.Second, "1s ago"},
		{4 * time.Second, "4s ago"},
		{59 * time.Second, "59s ago"},
		{time.Minute, "1m ago"},
		{11 * time.Minute, "11m ago"},
		{59*time.Minute + 59*time.Second, "59m ago"},
		{time.Hour, "1h ago"},
		{3 * time.Hour, "3h ago"},
		{23*time.Hour + 59*time.Minute, "23h ago"},
		{24 * time.Hour, "1d ago"},
		{49 * time.Hour, "2d ago"},
	}
	for _, c := range cases {
		if got := agoText(c.d); got != c.want {
			t.Errorf("agoText(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestElapsedText(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0h00m"},
		{40 * time.Minute, "0h40m"},
		{2*time.Hour + 4*time.Minute, "2h04m"},
		{26*time.Hour + 59*time.Minute + 59*time.Second, "26h59m"},
	}
	for _, c := range cases {
		if got := elapsedText(c.d); got != c.want {
			t.Errorf("elapsedText(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestTokensText(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{812, "812"},
		{999, "999"},
		{1000, "1k"},
		{45_400, "45k"},
		{312_000, "312k"},
		{999_499, "999k"},
		{999_500, "1.0M"},
		{1_000_000, "1.0M"},
		{1_240_000, "1.2M"},
		{60_000_000, "60.0M"},
		{116_100_000, "116.1M"},
	}
	for _, c := range cases {
		if got := tokensText(c.n); got != c.want {
			t.Errorf("tokensText(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestContextSizeText(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{{1_000_000, "1M"}, {2_000_000, "2M"}, {200_000, "200k"}, {1_500_000, "1.5M"}}
	for _, c := range cases {
		if got := ctxSizeText(c.n); got != c.want {
			t.Errorf("ctxSizeText(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestCellMillions(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "-"},
		{1, "<0.1M"},
		{49_999, "<0.1M"},
		{50_000, "0.1M"},
		{400_180, "0.4M"},
		{6_206_000, "6.2M"},
		{10_498_000, "10.5M"},
	}
	for _, c := range cases {
		if got := cellMillions(c.n); got != c.want {
			t.Errorf("cellMillions(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestPercentOf(t *testing.T) {
	cases := []struct {
		part, total float64
		want        int
	}{
		{47.10, 71.20, 66},
		{9.26, 71.20, 13},
		{2.02, 71.20, 3},
		{0, 71.20, 0},
		{5, 0, 0},
		{5, -1, 0},
	}
	for _, c := range cases {
		if got := percentOf(c.part, c.total); got != c.want {
			t.Errorf("percentOf(%v, %v) = %d, want %d", c.part, c.total, got, c.want)
		}
	}
}

func TestDaysSinceCountsWholeDaysAndIgnoresUnknownTimes(t *testing.T) {
	cases := []struct {
		name string
		at   time.Time
		want int
	}{
		{"unknown", time.Time{}, 0},
		{"just now", testNow, 0},
		{"almost a day", testNow.Add(-23 * time.Hour), 0},
		{"a day", testNow.Add(-24 * time.Hour), 1},
		{"a week and a half day", testNow.Add(-(7*24 + 12) * time.Hour), 7},
		{"in the future", testNow.Add(time.Hour), 0},
	}
	for _, c := range cases {
		if got := daysSince(c.at, testNow); got != c.want {
			t.Errorf("%s: daysSince = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestDaysLeftCountsAPartOfADayAsOne(t *testing.T) {
	cases := []struct {
		name string
		at   time.Time
		want int
	}{
		{"passed", testNow.Add(-time.Hour), 0},
		{"right now", testNow, 0},
		{"a minute", testNow.Add(time.Minute), 1},
		{"a day", testNow.Add(24 * time.Hour), 1},
		{"a day and an hour", testNow.Add(25 * time.Hour), 2},
		{"23 days", testNow.Add(23 * 24 * time.Hour), 23},
		{"22 and a half days", testNow.Add(22*24*time.Hour + 12*time.Hour), 23},
	}
	for _, c := range cases {
		if got := daysLeft(c.at, testNow); got != c.want {
			t.Errorf("%s: daysLeft = %d, want %d", c.name, got, c.want)
		}
	}
}
