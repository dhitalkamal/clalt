package tui

import (
	"fmt"
	"strings"
	"time"
)

// humanTokens renders a token count compactly (1.2M, 340.0K, 42).
func humanTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// humanCount renders a plain count with thousands separators (3,337).
func humanCount(n int) string {
	return withCommas(fmt.Sprintf("%d", n))
}

// withCommas inserts thousands separators into a plain integer string.
func withCommas(s string) string {
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// money renders a USD amount with a thousands-separated integer part ($17,742.11).
func money(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	dot := strings.IndexByte(s, '.')
	return "$" + withCommas(s[:dot]) + s[dot:]
}

// humanUntil renders time until t as "in 4h12m" / "expired 3m ago" / "unknown".
func humanUntil(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Until(t)
	if d < 0 {
		return "expired " + compactDur(-d) + " ago"
	}
	return "in " + compactDur(d)
}

func compactDur(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h >= 24 {
		return fmt.Sprintf("%dd %dh", h/24, h%24)
	}
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// trunc shortens s to at most n runes, marking a cut with a trailing tilde.
func trunc(s string, n int) string {
	if n < 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "~"
}

// dashPlan renders a plan label, or a dash when unset.
func dashPlan(p string) string {
	if p == "" {
		return "-"
	}
	return p
}

// shortModel trims the long model id to a friendly label (opus 4.8, sonnet 5).
func shortModel(id string) string {
	switch {
	case contains(id, "opus"):
		return "opus 4.8"
	case contains(id, "sonnet"):
		return "sonnet 5"
	case contains(id, "haiku"):
		return "haiku 4.5"
	case id == "":
		return "unknown"
	default:
		return id
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
