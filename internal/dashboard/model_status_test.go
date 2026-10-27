package dashboard

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestAFailedRefreshKeepsTheOldDataAndShowsTheError(t *testing.T) {
	clock := newClock()
	src := &fakeSource{snap: fiveSnapshot()}
	m := loadedModel(src, &fakeActions{}, clockOptions(clock), 100, 30)
	src.set(Snapshot{}, errors.New("state file is unreadable"))
	clock.advance(6 * time.Second)
	var cmd tea.Cmd
	m, cmd = upd(m, tickMsg{})
	next := Settle(m, cmd)
	out := Screen(next)
	if !strings.Contains(out, "update failed: state file is unreadable") {
		t.Errorf("the error shows in the header:\n%s", out)
	}
	if !strings.Contains(out, "morgan") || strings.Contains(out, "updated") {
		t.Errorf("the last good data stays and the updated age is replaced:\n%s", out)
	}
	if !strings.Contains(Raw(next), "update failed") {
		t.Error("sanity: the raw view has the error too")
	}

	src.set(fiveSnapshot(), nil)
	clock.advance(6 * time.Second)
	m2 := next.(model)
	m2, cmd = upd(m2, tickMsg{})
	if out := Screen(Settle(m2, cmd)); strings.Contains(out, "update failed") || !strings.Contains(out, "updated") {
		t.Errorf("the next good load clears the error:\n%s", out)
	}
}

func TestAgesAdvanceWithTheClockBetweenLoads(t *testing.T) {
	clock := newClock()
	clock.t = testNow
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, clockOptions(clock), 100, 30)
	if out := Screen(m); !strings.Contains(out, "Last used 4m ago.") || !strings.Contains(out, "Limits as of 4s ago.") {
		t.Fatalf("setup: the fixture ages at the load:\n%s", out)
	}
	clock.advance(10 * time.Minute)
	if out := Screen(m); !strings.Contains(out, "Last used 14m ago.") || !strings.Contains(out, "Limits as of 10m ago.") {
		t.Errorf("ten minutes later the same snapshot reads older:\n%s", out)
	}
}

func TestTheErrorIsCutToFitTheHeaderAndKeepsTheAge(t *testing.T) {
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, testOptions(), 60, 20)
	m, _ = upd(m, loadedMsg{err: errors.New(strings.Repeat("very long reason ", 20))})
	header := strings.Split(Screen(m), "\n")[0]
	if len(header) > 60 || !strings.Contains(header, "update failed: very long") || !strings.Contains(header, "... (data from") || !strings.HasSuffix(header, ")") {
		t.Errorf("the header keeps the start of the error, an ellipsis and how old the data is: %q", header)
	}
}

func TestAFailedRefreshSaysHowOldTheDataIs(t *testing.T) {
	clock := newClock()
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, clockOptions(clock), 100, 30)
	clock.advance(4 * time.Minute)
	m, _ = upd(m, loadedMsg{err: errors.New("state file is unreadable"), at: clock.now()})
	if header := strings.Split(Screen(m), "\n")[0]; !strings.HasSuffix(header, "update failed: state file is unreadable (data from 4m ago)") {
		t.Errorf("the age is the age of the last good read, not of the failed one: %q", header)
	}
}

func TestTheFailedHeaderDropsTheReasonBeforeTheAge(t *testing.T) {
	clock := newClock()
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, clockOptions(clock), 100, 30)
	clock.advance(4 * time.Minute)
	m, _ = upd(m, loadedMsg{err: errors.New(strings.Repeat("long reason ", 30)), at: clock.now()})
	sawAge, sawBare := false, false
	for w := 140; w >= 10; w-- {
		m.w = w
		line := plainText(m.headerLine())
		if len(line) > w {
			t.Fatalf("w=%d: the header is %d wide: %q", w, len(line), line)
		}
		switch {
		case strings.HasSuffix(line, "(data from 4m ago)"):
			sawAge = true
		case strings.HasSuffix(strings.TrimSpace(line), "update failed"):
			sawBare = true
		}
		if sawBare && strings.Contains(line, "(data") {
			t.Fatalf("w=%d: the age came back after it was dropped: %q", w, line)
		}
	}
	if !sawAge || !sawBare {
		t.Errorf("wide headers carry the age and the narrowest only say it failed (age %v, bare %v)", sawAge, sawBare)
	}
	m.w = 44
	if got := strings.Join(strings.Fields(plainText(m.headerLine())), " "); got != "clalt update failed (data from 4m ago)" {
		t.Errorf("when the reason will not fit the age is kept: %q", got)
	}
}

func TestStaleLimitsTurnYellowAfterFailedRefreshesToo(t *testing.T) {
	clock := newClock()
	clock.t = testNow
	opt := clockOptions(clock)
	opt.NoColor = false
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, opt, 100, 30)
	m, _ = upd(m, loadedMsg{err: errors.New("down"), at: clock.now()})
	if strings.Contains(Raw(m), "\x1b[33mLimits as of") {
		t.Fatal("setup: fresh limits are not yellow")
	}
	clock.advance(time.Hour)
	m, _ = upd(m, loadedMsg{err: errors.New("still down"), at: clock.now()})
	if raw := Raw(m); !strings.Contains(raw, "\x1b[33mLimits as of 1h ago.") {
		t.Errorf("an hour of failed refreshes makes the old reading yellow:\n%q", raw)
	}
}

func TestOnlyTheFirstLineOfAMultiLineErrorShows(t *testing.T) {
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, testOptions(), 100, 30)
	m, _ = upd(m, loadedMsg{err: errors.New("first line\nsecond line")})
	if out := Screen(m); !strings.Contains(out, "update failed: first line") || strings.Contains(out, "second line") {
		t.Errorf("one line in the header:\n%s", out)
	}
}

func TestFailingFirstLoadFillsThePaneWithTheError(t *testing.T) {
	m := newModel(context.Background(), &fakeSource{}, &fakeActions{}, testOptions())
	m, _ = upd(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m, _ = upd(m, loadedMsg{err: errors.New("permission denied")})
	out := Screen(m)
	for _, want := range []string{"Could not load accounts: permission denied", "load failed", "Trying again in the background."} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in\n%s", want, out)
		}
	}
	m, _ = upd(m, loadedMsg{snap: fiveSnapshot()})
	if out := Screen(m); strings.Contains(out, "Could not load") || !strings.Contains(out, "morgan") {
		t.Errorf("a later good load recovers:\n%s", out)
	}
}

func TestUpdatedAgeCountsUp(t *testing.T) {
	clock := newClock()
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, clockOptions(clock), 100, 30)
	for _, c := range []struct {
		after time.Duration
		want  string
	}{{0, "updated just now"}, {4 * time.Second, "updated 4s ago"}, {86 * time.Second, "updated 1m ago"}} {
		clock.advance(c.after)
		if out := Screen(m); !strings.Contains(out, c.want) {
			t.Errorf("%v after the load: want %q in\n%s", c.after, c.want, strings.Split(out, "\n")[0])
		}
	}
}

func TestASlowRefreshShowsUpdating(t *testing.T) {
	clock := newClock()
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, clockOptions(clock), 100, 30)
	clock.advance(6 * time.Second)
	m, _ = upd(m, tickMsg{})
	if out := Screen(m); strings.Contains(out, "updating") {
		t.Error("a refresh that just started is not slow yet")
	}
	clock.advance(2 * time.Second)
	if out := Screen(m); !strings.Contains(out, "updating...") {
		t.Errorf("after a second the header says it is updating:\n%s", strings.Split(out, "\n")[0])
	}
}

func TestSelectionFollowsTheAccountIDAcrossLoads(t *testing.T) {
	src := &fakeSource{snap: fiveSnapshot()}
	m := loadedModel(src, &fakeActions{}, testOptions(), 100, 30)
	next, _ := Press(m, "j", "j")
	m = next.(model)
	if m.sel != "a3" {
		t.Fatalf("setup: casey has id a3, selected %q", m.sel)
	}
	reordered := fiveSnapshot()
	accts := reordered.Accounts
	reordered.Accounts = []Account{accts[4], accts[3], accts[2], accts[1], accts[0]}
	m, _ = upd(m, loadedMsg{snap: reordered})
	if m.sel != "a3" || !strings.Contains(strings.Split(Screen(m), "\n")[1], "+- casey ") {
		t.Errorf("a reorder keeps casey selected, got %q:\n%s", m.sel, Screen(m))
	}
	gone := Snapshot{Accounts: []Account{accts[0], accts[1]}}
	m, _ = upd(m, loadedMsg{snap: gone})
	if m.sel != "a1" || !strings.Contains(strings.Split(Screen(m), "\n")[1], "+- robin ") {
		t.Errorf("a removed account moves the selection to the first one, got %q", m.sel)
	}
}

func TestNoAccountsAtAll(t *testing.T) {
	m := loadedModel(&fakeSource{snap: Snapshot{}}, &fakeActions{}, testOptions(), 100, 30)
	out := Screen(m)
	if !strings.Contains(out, "No accounts yet.") || !strings.Contains(out, "clalt login") {
		t.Errorf("an empty snapshot says how to add an account:\n%s", out)
	}
	for _, key := range []string{"enter", "s", "e", "x", "j", "k", "tab", "[", "]", "/", "?", "a", "h", "l"} {
		next, _ := Press(m, key)
		_ = Screen(next)
	}
}
