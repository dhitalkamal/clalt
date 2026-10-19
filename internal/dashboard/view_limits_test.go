package dashboard

import (
	"strings"
	"testing"
)

func TestLimitsViewLayout(t *testing.T) {
	c := limitsContent(ctxFor(wideBody), testAccount())
	lines := linesOf(c)
	first := lines[0]
	if !strings.HasPrefix(first, "5h window, last 7 days") || !strings.HasSuffix(first, "now 26%") {
		t.Errorf("the first line names the window and ends with the current percent: %q", first)
	}
	if n := len(c.body); n != 1+8+1+2 {
		t.Errorf("heading, 6 chart rows plus axis and days, a blank and two pace lines, got %d lines:\n%s", n, strings.Join(lines, "\n"))
	}
	if lineWith(lines, "7d  42% now, on pace for 71% by Fri 15:45") == "" {
		t.Errorf("missing the 7d pace sentence:\n%s", strings.Join(lines, "\n"))
	}
	if lineWith(lines, "5h  runs out around 01:10, about 35 minutes before it resets") == "" {
		t.Errorf("missing the 5h pace sentence:\n%s", strings.Join(lines, "\n"))
	}
	if strings.Index(strings.Join(lines, "\n"), "7d  ") > strings.Index(strings.Join(lines, "\n"), "5h  runs") {
		t.Error("the 7d sentence comes before the 5h sentence")
	}
	if !strings.Contains(lineWith(lines, "Thu"), "Wed") {
		t.Error("the day labels come from the snapshot")
	}
}

func TestLimitsChartHeightFollowsTheRoom(t *testing.T) {
	cases := []struct{ h, rows int }{{40, 8}, {24, 8}, {14, 8}, {12, 6}, {11, 5}, {5, 5}}
	for _, c := range cases {
		ctx := ctxFor(wideBody)
		ctx.h = c.h
		// 1 heading, chart rows, a blank and two pace lines
		got := len(limitsContent(ctx, testAccount()).body) - 4
		if got != c.rows {
			t.Errorf("view height %d: %d chart rows, want %d", c.h, got, c.rows)
		}
	}
}

func TestLimitsNowPercentTone(t *testing.T) {
	a := testAccount()
	a.Limits.Five.Percent = 95
	if sp := spanContaining(limitsContent(ctxFor(wideBody), a), "95%"); sp.t != toneBad {
		t.Errorf("95%% now is red, got tone %d", sp.t)
	}
}

func TestLimitsPaceSentencesTakeTheirLevel(t *testing.T) {
	c := limitsContent(ctxFor(wideBody), testAccount())
	if sp := spanContaining(c, "runs"); sp.t != toneWarn {
		t.Errorf("a warn level colors the 5h sentence, got tone %d", sp.t)
	}
	if sp := spanContaining(c, "pace"); sp.t != toneText {
		t.Errorf("an ok sentence keeps the default tone, got tone %d", sp.t)
	}
}

func TestLimitsWithoutReadings(t *testing.T) {
	a := testAccount()
	a.Limits.Readings = nil
	out := joined(limitsContent(ctxFor(wideBody), a))
	if !strings.Contains(out, "No readings yet.") || !strings.Contains(out, "5h  runs out") {
		t.Errorf("no readings says so and keeps the sentences:\n%s", out)
	}
}

func TestLimitsSignedOut(t *testing.T) {
	if got := linesOf(limitsContent(ctxFor(wideBody), signedOutAccount())); len(got) != 1 || got[0] != "Signed out. Press s to sign in." {
		t.Errorf("signed out limits = %q", got)
	}
}

func TestLimitsNeverOverflow(t *testing.T) {
	for w := 43; w <= 140; w++ {
		for _, h := range []int{12, 20, 40} {
			ctx := ctxFor(w)
			ctx.h = h
			for i, l := range linesOf(limitsContent(ctx, testAccount())) {
				if len(l) > w {
					t.Fatalf("w=%d h=%d line %d is %d wide: %q", w, h, i, len(l), l)
				}
			}
		}
	}
}
