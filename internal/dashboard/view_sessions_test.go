package dashboard

import (
	"strings"
	"testing"
	"time"
)

func TestSessionRowsShowTheApprovedDetails(t *testing.T) {
	lines := linesOf(sessionsContent(ctxFor(wideBody), testAccount()))
	rows := []string{
		"> working  ~/Projects/personal/clalt (feat/dashboard-v2)",
		"           Opus 5.5 high   context 312k of 1M   ",
		"           2h04m   +156 -23 lines   ~$1.10",
		"  waiting  ~/Projects/work/web (develop)",
		"           Sonnet 5.5 medium   context 48k of 1M   ",
		"           0h40m   +12 -3 lines   ~$0.62",
		"x stops the selected session.",
	}
	for _, r := range rows {
		if lineWith(lines, r) == "" {
			t.Errorf("missing session row %q in\n%s", r, strings.Join(lines, "\n"))
		}
	}
}

func TestSessionContentGeometry(t *testing.T) {
	c := sessionsContent(ctxFor(wideBody), testAccount())
	if c.rows != 2 || c.stride != 4 || c.span != 3 || c.first != 0 {
		t.Errorf("two sessions of three lines with a blank between: %+v", c)
	}
}

func TestSessionCursorMarkerFollowsTheRow(t *testing.T) {
	ctx := ctxFor(wideBody)
	ctx.row = 1
	out := joined(sessionsContent(ctx, testAccount()))
	if !strings.Contains(out, "> waiting") || strings.Contains(out, "> working") {
		t.Errorf("row 1 carries the marker:\n%s", out)
	}
}

func TestFocusMakesTheCursorRowBold(t *testing.T) {
	marker := func(focused bool) tone {
		ctx := ctxFor(wideBody)
		ctx.focused = focused
		return sessionsContent(ctx, testAccount()).body[0][0].t
	}
	if marker(true) != toneBold || marker(false) != toneDim {
		t.Error("the marker is bold with focus and dim without")
	}
}

func TestStateWordsKeepTheirTones(t *testing.T) {
	c := sessionsContent(ctxFor(wideBody), testAccount())
	if sp := spanContaining(c, "working"); sp.t != toneGood {
		t.Errorf("working should be green, got tone %d", sp.t)
	}
	if sp := spanContaining(c, "waiting"); sp.t != toneWarn {
		t.Errorf("waiting should be yellow, got tone %d", sp.t)
	}
	a := testAccount()
	a.Sessions[0].State = StateIdle
	if sp := spanContaining(sessionsContent(ctxFor(wideBody), a), "idle"); sp.t != toneDim {
		t.Errorf("idle should be dim, got tone %d", sp.t)
	}
}

func TestLongSessionLineEndsInAnEllipsis(t *testing.T) {
	out := joined(sessionsContent(ctxFor(43), testAccount()))
	if !strings.Contains(out, "...") {
		t.Errorf("a session line wider than the pane should be cut with an ellipsis:\n%s", out)
	}
	if !strings.Contains(out, "~/Projects/personal/clalt") {
		t.Error("the path should survive, the branch is what gets cut")
	}
	uni := ctxFor(43)
	uni.g = unicodeGlyphs()
	if !strings.Contains(joined(sessionsContent(uni, testAccount())), cp(0x2026)) {
		t.Error("unicode mode cuts with a real ellipsis glyph")
	}
}

// barTone is the tone of the first bar fill span in a line, dim if there is none
func barTone(line rich) tone {
	for _, sp := range line {
		if sp.s != "" && strings.Trim(sp.s, "#=") == "" {
			return sp.t
		}
	}
	return toneDim
}

func TestContextBarColorFollowsTheFill(t *testing.T) {
	a := testAccount()
	a.Sessions[0].ContextUsed = 912_000
	c := sessionsContent(ctxFor(wideBody), a)
	// line 1 is the first session's detail line, line 5 the second's
	if got := barTone(c.body[1]); got != toneBad {
		t.Errorf("91%% context fills red, got tone %d", got)
	}
	if got := barTone(c.body[5]); got != toneGood {
		t.Errorf("5%% context fills green, got tone %d", got)
	}
}

func TestSessionDetailDegradesBeforeOverflowing(t *testing.T) {
	for w := 43; w <= 140; w++ {
		for i, l := range linesOf(sessionsContent(ctxFor(w), testAccount())) {
			if len(l) > w {
				t.Fatalf("w=%d line %d is %d wide: %q", w, i, len(l), l)
			}
		}
	}
	tight := joined(sessionsContent(ctxFor(43), testAccount()))
	if !strings.Contains(tight, "Opus 5.5 high") {
		t.Errorf("the model survives a tight pane:\n%s", tight)
	}
}

func TestSessionsEmptyStates(t *testing.T) {
	if got := linesOf(sessionsContent(ctxFor(wideBody), signedOutAccount())); len(got) != 1 || got[0] != "Signed out. Press s to sign in." {
		t.Errorf("signed out sessions = %q", got)
	}
	a := testAccount()
	a.Sessions = nil
	c := sessionsContent(ctxFor(wideBody), a)
	if got := joined(c); got != "No sessions running. Press enter to start claude as robin." {
		t.Errorf("no sessions = %q", got)
	}
	if c.rows != 0 {
		t.Error("an empty list has no cursor rows")
	}
}

func TestSessionWithoutEffortBranchOrContext(t *testing.T) {
	a := testAccount()
	a.Sessions = []Session{{ID: "x", State: StateIdle, Folder: "~/tmp", Model: "Sonnet 5.5", StartedAt: testNow}}
	got := linesOf(sessionsContent(ctxFor(wideBody), a))[:3]
	want := []string{
		"> idle     ~/tmp",
		"           Sonnet 5.5",
		"           0h00m   ~$0.00",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSessionElapsedFollowsTheClock(t *testing.T) {
	c := ctxFor(wideBody)
	c.now = testNow.Add(time.Minute)
	out := joined(sessionsContent(c, testAccount()))
	if !strings.Contains(out, "2h05m   +156") || !strings.Contains(out, "0h41m   +12") {
		t.Errorf("a minute later both sessions are a minute older:\n%s", out)
	}
	a := testAccount()
	a.Sessions[0].StartedAt = time.Time{}
	if out := joined(sessionsContent(ctxFor(wideBody), a)); !strings.Contains(out, "-   +156 -23 lines") {
		t.Errorf("an unknown start shows a dash:\n%s", out)
	}
}
