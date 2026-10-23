package pick

import (
	"math"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

func win(used float64, resets time.Duration) Window {
	return Window{Used: used, ResetsAt: now.Add(resets), Known: true}
}

func TestRoomIsWhatIsLeftOfTheMoreUsedWindow(t *testing.T) {
	cases := []struct {
		name string
		l    Limits
		want float64
	}{
		{"the five-hour window is the tighter one", Limits{FiveHour: win(40, time.Hour), SevenDay: win(10, 48*time.Hour)}, 60},
		{"the seven-day window is the tighter one", Limits{FiveHour: win(10, time.Hour), SevenDay: win(75, 48*time.Hour)}, 25},
		{"only the five-hour window is known", Limits{FiveHour: win(30, time.Hour)}, 70},
		{"only the seven-day window is known", Limits{SevenDay: win(30, time.Hour)}, 70},
		{"nothing used", Limits{FiveHour: win(0, time.Hour), SevenDay: win(0, time.Hour)}, 100},
		{"a full window", Limits{FiveHour: win(100, time.Hour), SevenDay: win(20, time.Hour)}, 0},
	}
	for _, c := range cases {
		got, ok := c.l.Room(now)
		if !ok || got != c.want {
			t.Errorf("%s: Room = %v, %v, want %v", c.name, got, ok, c.want)
		}
	}
}

func TestRoomIsUnknownWhenNoWindowIs(t *testing.T) {
	if got, ok := (Limits{}).Room(now); ok {
		t.Errorf("Room of nothing = %v, true", got)
	}
	// a number that is not one is no data
	bad := Limits{FiveHour: Window{Used: math.NaN(), Known: true}}
	if got, ok := bad.Room(now); ok {
		t.Errorf("Room of NaN = %v, true", got)
	}
}

// the percentage only moves with use, and use through claude is reported, so an
// old figure holds until the window it belongs to has reset: after that the
// window is empty again
func TestAWindowThatHasResetCountsAsEmpty(t *testing.T) {
	l := Limits{FiveHour: win(95, -time.Hour), SevenDay: win(30, 48*time.Hour)}
	if got, ok := l.Room(now); !ok || got != 70 {
		t.Errorf("Room = %v, %v, want 70: the five-hour window reset an hour ago", got, ok)
	}
	both := Limits{FiveHour: win(95, -time.Hour), SevenDay: win(80, -time.Minute)}
	if got, ok := both.Room(now); !ok || got != 100 {
		t.Errorf("Room = %v, %v, want 100: both windows reset", got, ok)
	}
	// the moment of the reset is the reset
	edge := Limits{FiveHour: Window{Used: 90, ResetsAt: now, Known: true}}
	if got, _ := edge.Room(now); got != 100 {
		t.Errorf("Room at the reset time = %v, want 100", got)
	}
	// a window with no reset time cannot be told to be over
	unknown := Limits{FiveHour: Window{Used: 90, Known: true}}
	if got, _ := unknown.Room(now); got != 10 {
		t.Errorf("Room with no reset time = %v, want 10", got)
	}
}

func TestUsedIsKeptBetweenZeroAndAHundred(t *testing.T) {
	over := Limits{FiveHour: win(130, time.Hour)}
	under := Limits{FiveHour: win(-5, time.Hour)}
	if got, _ := over.Room(now); got != 0 {
		t.Errorf("Room of 130%% used = %v, want 0", got)
	}
	if got, _ := under.Room(now); got != 100 {
		t.Errorf("Room of -5%% used = %v, want 100", got)
	}
}

func TestLimitsAreRecentWithinTheMaxAge(t *testing.T) {
	seen := func(ago time.Duration) Limits { return Limits{ObservedAt: now.Add(-ago)} }
	cases := []struct {
		name string
		l    Limits
		want bool
	}{
		{"just now", seen(0), true},
		{"an hour ago", seen(time.Hour), true},
		{"exactly the max age", seen(DefaultMaxAge), true},
		{"older", seen(DefaultMaxAge + time.Second), false},
		{"never observed", Limits{}, false},
		// a clock that is a little ahead of this one
		{"in the future", seen(-time.Minute), true},
	}
	for _, c := range cases {
		if got := c.l.Recent(now, DefaultMaxAge); got != c.want {
			t.Errorf("%s: Recent = %v, want %v", c.name, got, c.want)
		}
	}
	if !seen(2*time.Hour).Recent(now, 3*time.Hour) || seen(4*time.Hour).Recent(now, 3*time.Hour) {
		t.Error("Recent ignores the max age it is given")
	}
}

func TestFiveHourUsedFollowsTheSameRulesAsRoom(t *testing.T) {
	if got, ok := (Limits{FiveHour: win(93, time.Hour)}).FiveHourUsed(now); !ok || got != 93 {
		t.Errorf("FiveHourUsed = %v, %v, want 93", got, ok)
	}
	if got, ok := (Limits{FiveHour: win(93, -time.Hour)}).FiveHourUsed(now); !ok || got != 0 {
		t.Errorf("after the reset: %v, %v, want 0", got, ok)
	}
	if _, ok := (Limits{SevenDay: win(50, time.Hour)}).FiveHourUsed(now); ok {
		t.Error("an unknown five-hour window has a figure")
	}
}
