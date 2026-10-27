package dashboard

import (
	"testing"
	"time"
)

// win is a window whose times are offsets from testNow, 23:53 on Wed 15 Oct
func win(pct, projected int, resetsIn, runsOutIn time.Duration) Window {
	w := Window{Percent: pct, Projected: projected, ResetsAt: testNow.Add(resetsIn)}
	if runsOutIn > 0 {
		w.RunsOutAt = testNow.Add(runsOutIn)
	}
	return w
}

const (
	hr = time.Hour
	mn = time.Minute
)

func TestPaceShortSentences(t *testing.T) {
	cases := []struct {
		name   string
		w      Window
		weekly bool
		want   string
	}{
		{"5h runs out later", win(26, 135, 1*hr+52*mn, 1*hr+17*mn), false, "at this pace it runs out around 01:10"},
		{"5h runs out soon", win(95, 180, 48*mn, 5*mn), false, "at this pace it runs out in about 5 minutes"},
		{"5h runs out within a minute", win(99, 200, 48*mn, 30*time.Second), false, "at this pace it runs out in under a minute"},
		{"5h runs out in one minute", win(99, 200, 48*mn, mn+10*time.Second), false, "at this pace it runs out in about 1 minute"},
		{"5h runs out just under an hour", win(90, 150, 2*hr, 59*mn+50*time.Second), false, "at this pace it runs out in about 59 minutes"},
		{"5h runs out in an hour", win(90, 150, 2*hr, hr), false, "at this pace it runs out around 00:53"},
		{"5h has room", win(22, 41, 2*hr+17*mn, 0), false, "on pace to end this window at 41%"},
		{"5h has lots of room", win(5, 12, 4*hr+37*mn, 0), false, "on pace to end this window at 12%"},
		{"5h over 100 without a time", win(50, 120, 2*hr, 0), false, "at this pace it runs out before it resets"},
		{"5h no forecast", win(0, 0, 2*hr, 0), false, ""},
		{"7d on pace", win(42, 71, 39*hr+52*mn, 0), true, "on pace to finish the week at 71%"},
		{"7d runs out later", win(80, 130, 39*hr+52*mn, 30*hr), true, "at this pace it runs out around Fri 05:53"},
		{"7d runs out soon", win(97, 130, 39*hr+52*mn, 40*mn), true, "at this pace it runs out in about 40 minutes"},
		{"7d no forecast", win(0, 0, 2*hr, 0), true, ""},
	}
	for _, c := range cases {
		if got := paceShort(c.w, c.weekly, testNow); got != c.want {
			t.Errorf("%s: paceShort = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPaceLongSentences(t *testing.T) {
	cases := []struct {
		name   string
		w      Window
		weekly bool
		want   string
	}{
		{"5h runs out later", win(26, 135, 1*hr+52*mn, 1*hr+17*mn), false, "runs out around 01:10, about 35 minutes before it resets"},
		{"5h runs out soon", win(95, 180, 48*mn+30*time.Second, 5*mn+15*time.Second), false, "runs out in about 5 minutes, 43 minutes before it resets"},
		{"5h runs out long before", win(60, 200, 4*hr+37*mn, 90*mn), false, "runs out around 01:23, about 3 hours before it resets"},
		{"5h over 100 without a time", win(50, 120, 2*hr, 0), false, "runs out before it resets"},
		{"5h no risk", win(5, 12, 4*hr+37*mn, 0), false, "on pace to end this window at 12%, no risk of running out"},
		{"5h plenty of room", win(22, 41, 2*hr+17*mn, 0), false, "on pace to end this window at 41%, plenty of room"},
		{"5h some room", win(40, 75, 2*hr, 0), false, "on pace to end this window at 75%, some room left"},
		{"5h close", win(60, 95, 2*hr, 0), false, "on pace to end this window at 95%, close to the limit"},
		{"5h no forecast", win(0, 0, 2*hr, 0), false, ""},
		{"7d on pace", win(42, 71, 39*hr+52*mn, 0), true, "42% now, on pace for 71% by Fri 15:45"},
		{"7d runs out", win(80, 130, 39*hr+52*mn, 30*hr), true, "runs out around Fri 05:53, about 10 hours before it resets"},
		{"7d runs out days before", win(80, 130, 5*24*hr, 2*24*hr), true, "runs out around Fri 23:53, about 3 days before it resets"},
		{"7d no forecast", win(0, 0, 2*hr, 0), true, ""},
	}
	for _, c := range cases {
		if got := paceLong(c.w, c.weekly, testNow); got != c.want {
			t.Errorf("%s: paceLong = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPaceLevel(t *testing.T) {
	cases := []struct {
		name string
		w    Window
		want Level
	}{
		{"runs out soon", win(95, 180, 48*mn, 5*mn), LevelBad},
		{"runs out in a quarter hour", win(90, 150, 2*hr, 15*mn), LevelWarn},
		{"runs out within a quarter hour", win(90, 150, 2*hr, 15*mn-time.Second), LevelBad},
		{"runs out later", win(26, 135, 1*hr+52*mn, 1*hr+17*mn), LevelWarn},
		{"over 100 without a time", win(50, 120, 2*hr, 0), LevelWarn},
		{"ends near the limit", win(80, 97, 2*hr, 0), LevelWarn},
		{"ends at 90", win(80, 90, 2*hr, 0), LevelWarn},
		{"ends at 89", win(80, 89, 2*hr, 0), LevelOK},
		{"plenty of room", win(22, 41, 2*hr, 0), LevelOK},
		{"no forecast", win(0, 0, 2*hr, 0), LevelOK},
	}
	for _, c := range cases {
		if got := paceLevel(c.w, testNow); got != c.want {
			t.Errorf("%s: paceLevel = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCountdownText(t *testing.T) {
	cases := []struct {
		d      time.Duration
		weekly bool
		want   string
	}{
		{1*hr + 52*mn + 30*time.Second, false, "in 1h52m"},
		{48*mn + 30*time.Second, false, "in 0h48m"},
		{4*hr + 37*mn, false, "in 4h37m"},
		{0, false, "in 0h00m"},
		{-mn, false, "in 0h00m"},
		{39*hr + 52*mn, true, "1d 15h"},
		{5*24*hr + 11*hr + 22*mn, true, "5d 11h"},
		{18*hr + 12*mn, true, "18h"},
		{40 * mn, true, "40m"},
		{-mn, true, "0m"},
	}
	for _, c := range cases {
		if got := countdownText(c.d, c.weekly); got != c.want {
			t.Errorf("countdownText(%v, weekly %v) = %q, want %q", c.d, c.weekly, got, c.want)
		}
	}
}

func TestResetClockUsesTheClocksZone(t *testing.T) {
	five := win(26, 41, 1*hr+52*mn, 0)
	seven := win(42, 71, 39*hr+52*mn, 0)
	if got := resetClock(five, false, testNow); got != "01:45" {
		t.Errorf("5h reset = %q, want a clock time", got)
	}
	if got := resetClock(seven, true, testNow); got != "Fri 15:45" {
		t.Errorf("7d reset = %q, want a weekday and a clock time", got)
	}
	west := time.FixedZone("west", -5*3600)
	local := testNow.In(west)
	if got := resetClock(five, false, local); got != "20:45" {
		t.Errorf("the reset shows in the clock's zone: %q", got)
	}
	if got := resetClock(seven, true, local); got != "Fri 10:45" {
		t.Errorf("the 7d reset shows in the clock's zone: %q", got)
	}
}
