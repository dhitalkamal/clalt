package dashboard

import (
	"strings"
	"time"
)

// the fixture follows the approved robin numbers where that is cheap. the
// demo package holds the full ledger, this one only feeds view rules.

// testNow is the clock the fixture is built against. the view tests pass it as
// the current time, so ages and countdowns read the same on every run.
var testNow = time.Date(2025, time.October, 15, 23, 53, 4, 0, time.UTC)

// testCosts are daily costs in dollars, newest first, 30 days
var testCosts = []float64{
	2.14, 3.62, 2.87, 0.95, 1.40, 4.05, 3.37, 2.70, 3.10, 2.55,
	1.75, 1.75, 3.40, 2.05, 2.30, 3.15, 2.60, 0.85, 1.10, 3.05,
	2.45, 2.95, 3.30, 2.75, 0.90, 0.60, 2.20, 3.10, 2.40, 1.80,
}

var testSessionsPerDay = []int{3, 4, 3, 1, 2, 5, 4, 3, 4, 3}

func testHistoryDays() []Day {
	anchor := time.Date(2025, time.October, 15, 0, 0, 0, 0, time.UTC)
	out := make([]Day, len(testCosts))
	for i, c := range testCosts {
		n := 2
		if i < len(testSessionsPerDay) {
			n = testSessionsPerDay[i]
		}
		out[i] = Day{
			Label:      anchor.AddDate(0, 0, -i).Format("Mon 2"),
			Input:      int64(c * 187_000),
			Output:     int64(c * 60_000),
			CacheRead:  int64(c * 2_900_000),
			CacheWrite: int64(c * 140_000),
			Cost:       c,
			Sessions:   n,
		}
	}
	return out
}

func testAccount() Account {
	return Account{
		ID: "robin", Label: "robin",
		Email: "robin@example.com", Org: "Example", Plan: "team plan",
		SignedIn:   true,
		Login:      Login{SignedInAt: testNow.Add(-7 * 24 * time.Hour), DueAt: testNow.Add(23 * 24 * time.Hour)},
		LastUsedAt: testNow.Add(-4 * time.Minute), LastIn: "~/Projects/personal/clalt",
		Limits: Limits{
			Five: Window{Percent: 26, ResetsAt: testNow.Add(1*time.Hour + 52*time.Minute), Projected: 135,
				RunsOutAt: testNow.Add(1*time.Hour + 17*time.Minute)},
			Seven:    Window{Percent: 42, ResetsAt: testNow.Add(39*time.Hour + 52*time.Minute), Projected: 71},
			ReadAt:   testNow.Add(-4 * time.Second),
			Readings: testReadings(),
			Days:     testDays,
		},
		Budget: Budget{Daily: 10, Weekly: 50},
		Sessions: []Session{
			{ID: "s1", State: StateWorking, Folder: "~/Projects/personal/clalt", Branch: "feat/dashboard-v2",
				Model: "Opus 5.5", Effort: "high", ContextUsed: 312_000, ContextSize: 1_000_000,
				StartedAt: testNow.Add(-(2*time.Hour + 4*time.Minute)), Added: 156, Removed: 23, Cost: 1.10},
			{ID: "s2", State: StateWaiting, Folder: "~/Projects/work/web", Branch: "develop",
				Model: "Sonnet 5.5", Effort: "medium", ContextUsed: 48_000, ContextSize: 1_000_000,
				StartedAt: testNow.Add(-40 * time.Minute), Added: 12, Removed: 3, Cost: 0.62},
		},
		History: History{
			Days: testHistoryDays(),
			Projects: []Share{
				{Name: "clalt", Tokens: 12_300_000, Cost: 47.10, Sessions: 40, Activity: []Activity{
					{Day: "Wed 15", Model: "Opus 5.5", Sessions: 2, Tokens: 340_000, Cost: 1.52},
					{Day: "Tue 14", Model: "Opus 5.5", Sessions: 3, Tokens: 900_000, Cost: 3.10},
					{Day: "Tue 14", Model: "Sonnet 5.5", Sessions: 1, Tokens: 210_000, Cost: 0.52},
					{Day: "Mon 13", Model: "Sonnet 5.5", Sessions: 3, Tokens: 1_300_000, Cost: 2.87},
					{Day: "Fri 10", Model: "Opus 5.5", Sessions: 5, Tokens: 1_600_000, Cost: 4.05},
					{Day: "Thu 9", Model: "Opus 5.5", Sessions: 4, Tokens: 12_000, Cost: 0.03},
				}},
				{Name: "web", Tokens: 2_400_000, Cost: 9.26, Sessions: 20, Activity: []Activity{
					{Day: "Sun 12", Model: "Sonnet 5.5", Sessions: 1, Tokens: 120_000, Cost: 0.95},
				}},
				{Name: "other", Tokens: 500_000, Cost: 14.84, Sessions: 6},
			},
			Models: []Share{
				{Name: "Opus 5.5", Tokens: 12_100_000, Cost: 49.10},
				{Name: "Sonnet 5.5", Tokens: 5_500_000, Cost: 22.10},
			},
		},
		Profile: Profile{Folder: "~/.config/clalt/profiles/robin", ClaudeVersion: "2.1.296", SharedSettings: true, FeedInstalled: true},
	}
}

func signedOutAccount() Account {
	return Account{ID: "harriet", Label: "harriet", Email: "harriet@example.com", Org: "Example"}
}

// ctxFor is a view context for a 100 column layout unless told otherwise
func ctxFor(w int) viewCtx {
	return viewCtx{g: asciiGlyphs(), w: w, h: 24, now: testNow}
}

// linesOf flattens a view's head and body to plain text lines
func linesOf(c content) []string {
	var out []string
	for _, r := range c.head {
		out = append(out, plainText(r))
	}
	for _, r := range c.body {
		out = append(out, plainText(r))
	}
	return out
}

func joined(c content) string { return strings.Join(linesOf(c), "\n") }

// lineWith returns the first line containing want, or fails the test through
// the caller's check on the empty string
func lineWith(lines []string, want string) string {
	for _, l := range lines {
		if strings.Contains(l, want) {
			return l
		}
	}
	return ""
}
