package dashboard

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// wideBody is the text width of the right pane at 100 columns
const wideBody = 67

func TestOverviewRowsAtTheWideWidth(t *testing.T) {
	out := joined(overviewContent(ctxFor(wideBody), testAccount()))
	rows := []string{
		"robin@example.com   Example   team plan",
		"Signed in. Re-login due in 23 days. Last used 4m ago.",
		"5h  ", "  26%  resets 01:45 (in 1h52m)",
		"    at this pace it runs out around 01:10",
		"7d  ", "  42%  resets Fri 15:45 (1d 15h)",
		"    on pace to finish the week at 71%",
		"Limits as of 4s ago.",
		"Budget   today  $2.14 of $10.00   ",
		"         week  $18.40 of $50.00   ",
		"Today    529k tokens   ~$2.14",
		"Week     *#::+*=   4.5M tokens   ~$18.40",
		"Running  2 sessions: 1 working, 1 waiting for you",
	}
	for _, r := range rows {
		if !strings.Contains(out, r) {
			t.Errorf("missing overview row %q in\n%s", r, out)
		}
	}
}

func TestBarsShareOneWidthAndFollowThePercent(t *testing.T) {
	lines := linesOf(overviewContent(ctxFor(wideBody), testAccount()))
	barRe := regexp.MustCompile(`[#=-]{10,}`)
	b5 := barRe.FindString(lineWith(lines, "  26%  resets"))
	b7 := barRe.FindString(lineWith(lines, "  42%  resets"))
	if len(b5) != len(b7) || len(b5) < 20 {
		t.Fatalf("both bars should share one width, got %d and %d", len(b5), len(b7))
	}
	full := func(b string) int { return strings.Count(b, "#") }
	if full(b5) >= full(b7) || full(b5) == 0 {
		t.Errorf("42%% should fill more than 26%%: %q vs %q", b7, b5)
	}
	if want := len(b5) * 26 / 100; full(b5) < want-1 || full(b5) > want+1 {
		t.Errorf("26%% of %d cells should be about %d filled, got %d", len(b5), want, full(b5))
	}
	if !strings.Contains(b5, "=") {
		t.Errorf("26%% leaves a partial cell, shown as a half cell: %q", b5)
	}
}

func TestStatusSplitsAtASentenceWhenNarrow(t *testing.T) {
	narrow := linesOf(overviewContent(ctxFor(51), testAccount()))
	if strings.Contains(lineWith(narrow, "Signed in. Re-login due in 23 days."), "Last used") {
		t.Error("at 51 columns the login sentence gets its own line")
	}
	if lineWith(narrow, "Last used 4m ago.") == "" {
		t.Error("the last used sentence still shows")
	}
	wide := linesOf(overviewContent(ctxFor(wideBody), testAccount()))
	if !strings.Contains(lineWith(wide, "Signed in."), "Last used 4m ago.") {
		t.Error("at 67 columns the status stays on one line")
	}
}

func TestResetTextShrinksBeforeTheBar(t *testing.T) {
	full := joined(overviewContent(ctxFor(wideBody), testAccount()))
	if !strings.Contains(full, "resets 01:45 (in 1h52m)") || !strings.Contains(full, "resets Fri 15:45 (1d 15h)") {
		t.Error("wide layout shows the full reset text")
	}
	tight := joined(overviewContent(ctxFor(43), testAccount()))
	if strings.Contains(tight, "(in") || strings.Contains(tight, "(1d") || !strings.Contains(tight, "resets 01:45") {
		t.Errorf("43 columns drops the countdown but keeps the time:\n%s", tight)
	}
	if !strings.Contains(tight, "resets Fri 15:45") {
		t.Error("the 7d reset time stays whole")
	}
}

func TestTodayAndWeekFollowTheDays(t *testing.T) {
	a := testAccount()
	a.History.Days = []Day{{Label: "Wed 15", Input: 1_000_000, Output: 200_000, Cost: 5}}
	out := joined(overviewContent(ctxFor(wideBody), a))
	for _, want := range []string{"Today    1.2M tokens   ~$5.00", "today $5.00 of $10.00", "week  $5.00 of $50.00", "   1.2M tokens   ~$5.00"} {
		if !strings.Contains(out, want) {
			t.Errorf("a one day history should show %q in\n%s", want, out)
		}
	}
	a.History.Days = nil
	out = joined(overviewContent(ctxFor(wideBody), a))
	if !strings.Contains(out, "Today    0 tokens   ~$0.00") {
		t.Errorf("no days means zero usage:\n%s", out)
	}
}

func TestBudgetRows(t *testing.T) {
	a := testAccount()
	a.Budget = Budget{}
	if out := joined(overviewContent(ctxFor(wideBody), a)); !strings.Contains(out, "Budget   not set. Press e to set one.") {
		t.Errorf("no caps should prompt for one:\n%s", out)
	}
	a.Budget = Budget{Weekly: 50}
	out := joined(overviewContent(ctxFor(wideBody), a))
	if !strings.Contains(out, "Budget   week  $18.40 of $50.00") || strings.Contains(out, "today  ") {
		t.Errorf("only the weekly cap is set:\n%s", out)
	}
	a.Budget = Budget{Daily: 1}
	if out := joined(overviewContent(ctxFor(wideBody), a)); !strings.Contains(out, "Budget   today $2.14 of $1.00") {
		t.Errorf("spend above the cap still shows:\n%s", out)
	}
}

func TestSignedOutOverviewShowsIdentityAndThePrompt(t *testing.T) {
	c := overviewContent(ctxFor(wideBody), signedOutAccount())
	lines := linesOf(c)
	if len(lines) != 2 || lines[0] != "harriet@example.com   Example" || lines[1] != "Signed out. Press s to sign in." {
		t.Errorf("signed out overview = %q", lines)
	}
}

func TestLoginToneFollowsTheDaysLeft(t *testing.T) {
	cases := []struct {
		days int
		want tone
		text string
	}{
		{23, toneDim, "Re-login due in 23 days."},
		{3, toneWarn, "Re-login due in 3 days."},
		{1, toneBad, "Re-login due in 1 day."},
		{0, toneBad, "Re-login is due now."},
	}
	for _, c := range cases {
		a := testAccount()
		a.Login.DueAt = testNow.Add(time.Duration(c.days) * 24 * time.Hour)
		sp := spanContaining(overviewContent(ctxFor(wideBody), a), "Re-login")
		if sp.s != c.text || sp.t != c.want {
			t.Errorf("due in %d days: span %q tone %d, want %q tone %d", c.days, sp.s, sp.t, c.text, c.want)
		}
	}
}

func TestOldLimitsTurnTheAgeYellow(t *testing.T) {
	a := testAccount()
	for age, want := range map[time.Duration]tone{4 * time.Second: toneDim, 59 * time.Minute: toneDim, time.Hour: toneWarn, 3 * time.Hour: toneWarn} {
		a.Limits.ReadAt = testNow.Add(-age)
		if sp := spanContaining(overviewContent(ctxFor(wideBody), a), "Limits as of"); sp.t != want {
			t.Errorf("limits %v old: tone %d, want %d", age, sp.t, want)
		}
	}
	a.Limits.ReadAt = testNow.Add(-2 * time.Hour)
	if out := joined(overviewContent(ctxFor(wideBody), a)); !strings.Contains(out, "Limits as of 2h ago.") {
		t.Errorf("age text missing:\n%s", out)
	}
}

func TestAgesAreMeasuredAgainstTheClockInTheContext(t *testing.T) {
	c := ctxFor(wideBody)
	c.now = testNow.Add(3 * time.Minute)
	out := joined(overviewContent(c, testAccount()))
	for _, want := range []string{"Last used 7m ago.", "Limits as of 3m ago."} {
		if !strings.Contains(out, want) {
			t.Errorf("three minutes later the overview should say %q:\n%s", want, out)
		}
	}
}

func TestLoginDueCountsPartialDaysUp(t *testing.T) {
	cases := []struct {
		left time.Duration
		text string
		want tone
	}{
		{25 * time.Hour, "Re-login due in 2 days.", toneWarn},
		{23 * time.Hour, "Re-login due in 1 day.", toneBad},
		{time.Minute, "Re-login due in 1 day.", toneBad},
		{-time.Hour, "Re-login is due now.", toneBad},
	}
	for _, c := range cases {
		a := testAccount()
		a.Login.DueAt = testNow.Add(c.left)
		sp := spanContaining(overviewContent(ctxFor(wideBody), a), "Re-login")
		if sp.s != c.text || sp.t != c.want {
			t.Errorf("due in %v: span %q tone %d, want %q tone %d", c.left, sp.s, sp.t, c.text, c.want)
		}
	}
}

func TestUnknownTimesSayWhatIsMissing(t *testing.T) {
	a := testAccount()
	a.LastUsedAt, a.Login, a.Limits.ReadAt = time.Time{}, Login{}, time.Time{}
	c := overviewContent(ctxFor(wideBody), a)
	out := joined(c)
	for _, want := range []string{"Signed in. Never used.", "No limit reading yet."} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "Re-login") || strings.Contains(out, "ago") {
		t.Errorf("an unknown time is not an age or a due date:\n%s", out)
	}
	if sp := spanContaining(c, "No limit"); sp.t != toneWarn {
		t.Errorf("a missing reading is worth a yellow word, got tone %d", sp.t)
	}
}

func TestPaceTextTakesTheWindowLevel(t *testing.T) {
	a := testAccount()
	if sp := spanContaining(overviewContent(ctxFor(wideBody), a), "runs"); sp.t != toneWarn {
		t.Errorf("a warn level colors the pace line, got tone %d", sp.t)
	}
	if sp := spanContaining(overviewContent(ctxFor(wideBody), a), "finish"); sp.t != toneDim {
		t.Errorf("an ok level leaves the pace line dim, got tone %d", sp.t)
	}
	a.Limits.Five.RunsOutAt = testNow.Add(5 * time.Minute)
	if sp := spanContaining(overviewContent(ctxFor(wideBody), a), "runs"); sp.t != toneBad {
		t.Errorf("a bad level colors the pace line red, got tone %d", sp.t)
	}
}

func TestOverviewShowsTheProblemInRedUnderTheIdentity(t *testing.T) {
	a := testAccount()
	a.Problem = "the limits feed is not installed, so limits are read from transcripts only"
	c := overviewContent(ctxFor(wideBody), a)
	lines := linesOf(c)
	if lines[0] != "robin@example.com   Example   team plan" || !strings.HasPrefix(lines[1], "! the limits feed is not installed") {
		t.Fatalf("the problem comes right after the identity:\n%s", strings.Join(lines[:4], "\n"))
	}
	if !strings.HasPrefix(lines[2], "  ") || strings.HasPrefix(lines[2], "! ") || !strings.Contains(lines[2], "transcripts only") {
		t.Errorf("a long problem wraps under its own text: %q", lines[2])
	}
	if sp := spanContaining(c, "limits"); sp.t != toneBad {
		t.Errorf("the problem is red, got tone %d", sp.t)
	}
	out := joined(overviewContent(ctxFor(wideBody), signedOutAccount()))
	if strings.Contains(out, "!") {
		t.Errorf("no problem, no marker:\n%s", out)
	}
	so := signedOutAccount()
	so.Problem = "profile folder is missing"
	if got := linesOf(overviewContent(ctxFor(wideBody), so)); len(got) != 3 || got[1] != "! profile folder is missing" || got[2] != "Signed out. Press s to sign in." {
		t.Errorf("a signed out account shows its problem too: %q", got)
	}
}

func TestRunningSummary(t *testing.T) {
	cases := []struct {
		sessions []Session
		want     string
	}{
		{nil, "Running  nothing right now"},
		{[]Session{{State: StateWaiting}}, "Running  1 session: 1 waiting for you"},
		{[]Session{{State: StateWorking}, {State: StateWorking}}, "Running  2 sessions: 2 working"},
		{[]Session{{State: StateWorking}, {State: StateWaiting}, {State: StateIdle}}, "Running  3 sessions: 1 working, 1 waiting for you, 1 idle"},
	}
	for _, c := range cases {
		a := testAccount()
		a.Sessions = c.sessions
		if out := joined(overviewContent(ctxFor(wideBody), a)); !strings.Contains(out, c.want) {
			t.Errorf("want %q in\n%s", c.want, out)
		}
	}
}

func TestOverviewNeverOverflowsItsWidth(t *testing.T) {
	// 43 is the narrowest right pane the layout produces
	for w := 43; w <= 140; w++ {
		for _, a := range []Account{testAccount(), signedOutAccount()} {
			for i, l := range linesOf(overviewContent(ctxFor(w), a)) {
				if len([]rune(l)) > w {
					t.Fatalf("w=%d line %d is %d wide: %q", w, i, len(l), l)
				}
			}
		}
	}
}

// spanContaining finds the first span whose text contains want. wrapped text is
// split into one span per word, so look for a single word.
func spanContaining(c content, want string) span {
	for _, r := range append(append([]rich(nil), c.head...), c.body...) {
		for _, sp := range r {
			if strings.Contains(sp.s, want) {
				return sp
			}
		}
	}
	return span{}
}
