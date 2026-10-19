package dashboard

import (
	"fmt"
	"math"
	"time"
)

func money(v float64) string  { return fmt.Sprintf("$%.2f", v) }
func approx(v float64) string { return fmt.Sprintf("~$%.2f", v) }

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func agoText(d time.Duration) string {
	switch {
	case d < time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d/time.Second))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d/time.Hour))
	}
	return fmt.Sprintf("%dd ago", int(d/(24*time.Hour)))
}

// daysSince counts whole days back from now. an unknown or future time is zero.
func daysSince(t, now time.Time) int {
	if t.IsZero() || t.After(now) {
		return 0
	}
	return int(now.Sub(t) / (24 * time.Hour))
}

// daysLeft counts the days up to t with a part of a day as a whole one, so a
// login due in 25 hours is 2 days away. zero once t has passed.
func daysLeft(t, now time.Time) int {
	d := t.Sub(now)
	if d <= 0 {
		return 0
	}
	return int((d + 24*time.Hour - 1) / (24 * time.Hour))
}

func elapsedText(d time.Duration) string {
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

// tokensText is the headline count: whole numbers under a thousand, then k,
// then millions with one decimal
func tokensText(n int64) string {
	switch {
	case n >= 999_500:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1000:
		return fmt.Sprintf("%dk", (n+500)/1000)
	}
	return fmt.Sprint(n)
}

// ctxSizeText writes a context window size, "1M" rather than "1.0M"
func ctxSizeText(n int64) string {
	if n > 0 && n%1_000_000 == 0 {
		return fmt.Sprintf("%dM", n/1_000_000)
	}
	return tokensText(n)
}

// cellMillions fills a table cell. zero is a dash and a small nonzero count
// says so instead of rounding down to nothing.
func cellMillions(n int64) string {
	switch {
	case n <= 0:
		return "-"
	case n < 50_000:
		return "<0.1M"
	}
	return fmt.Sprintf("%.1fM", float64(n)/1e6)
}

func percentOf(part, total float64) int {
	if total <= 0 {
		return 0
	}
	return int(math.Round(part / total * 100))
}
