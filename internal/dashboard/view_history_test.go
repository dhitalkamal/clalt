package dashboard

import (
	"strings"
	"testing"
)

func historyCtx(w, sub int) viewCtx {
	c := ctxFor(w)
	c.sub = sub
	return c
}

func TestDaysTableMatchesTheApprovedRows(t *testing.T) {
	lines := linesOf(historyContent(historyCtx(wideBody, 0), testAccount()))
	rows := []string{
		"[Days]  Projects  Models",
		"  day     input  output  cache rd  cache wr    cost  sessions",
		"> Wed 15   0.4M    0.1M      6.2M      0.3M  ~$2.14         3",
		"  Tue 14   0.7M    0.2M     10.5M      0.5M  ~$3.62         4",
		"  Sun 12   0.2M    0.1M      2.8M      0.1M  ~$0.95         1",
		"  Mon 6    0.5M    0.2M      7.4M      0.4M  ~$2.55         3",
	}
	for _, r := range rows {
		if lineWith(lines, r) == "" {
			t.Errorf("missing days row %q in\n%s", r, strings.Join(lines, "\n"))
		}
	}
}

func TestDaysTableShowsTenRowsAndTheThirtyDayLine(t *testing.T) {
	c := historyContent(historyCtx(wideBody, 0), testAccount())
	if c.rows != 10 || c.stride != 1 || c.span != 1 || c.first != 0 {
		t.Errorf("ten one line rows: %+v", c)
	}
	last := plainText(c.body[len(c.body)-1])
	if !strings.HasSuffix(last, "last 30 days  ~$71.20") {
		t.Errorf("the closing line has the 30 day total: %q", last)
	}
	if spark := strings.TrimSpace(strings.TrimSuffix(last, "last 30 days  ~$71.20")); len(spark) != 30 {
		t.Errorf("a wide pane draws one spark cell per day, got %d in %q", len(spark), spark)
	}
	if got := plainText(c.body[len(c.body)-2]); got != "" {
		t.Errorf("a blank line separates the table from the total, got %q", got)
	}
}

func TestThirtyDayTotalIsNeverCutOff(t *testing.T) {
	for w := 43; w <= 140; w++ {
		out := joined(historyContent(historyCtx(w, 0), testAccount()))
		if !strings.Contains(out, "last 30 days  ~$71.20") {
			t.Errorf("width %d lost the 30 day total:\n%s", w, out)
		}
	}
}

func TestDaysTableAbbreviatesWhenNarrow(t *testing.T) {
	wide := joined(historyContent(historyCtx(wideBody, 0), testAccount()))
	for _, h := range []string{"cache rd", "cache wr", "sessions"} {
		if !strings.Contains(wide, h) {
			t.Errorf("wide days table should say %q", h)
		}
	}
	narrow := joined(historyContent(historyCtx(43, 0), testAccount()))
	if strings.Contains(narrow, "cache rd") || !strings.Contains(narrow, "sess") {
		t.Errorf("narrow days table should use short headers:\n%s", narrow)
	}
	if !strings.Contains(narrow, "Wed 15") || !strings.Contains(narrow, "~$2.14") {
		t.Error("narrow days table still shows the rows")
	}
}

func TestQuietDaysAreDim(t *testing.T) {
	a := testAccount()
	a.History.Days[1] = Day{Label: "Tue 14"}
	c := historyContent(historyCtx(wideBody, 0), a)
	row := c.body[1]
	for _, sp := range row[2:] {
		if sp.t != toneDim && strings.TrimSpace(sp.s) != "" {
			t.Errorf("a day without usage is dim, found %q in tone %d", sp.s, sp.t)
		}
	}
}

func TestProjectsTable(t *testing.T) {
	c := historyContent(historyCtx(wideBody, 1), testAccount())
	lines := linesOf(c)
	if lineWith(lines, "Days  [Projects]  Models") == "" {
		t.Errorf("the projects tab is active:\n%s", strings.Join(lines, "\n"))
	}
	head := lineWith(lines, "project")
	for _, h := range []string{"share", "tokens", "cost", "sessions"} {
		if !strings.Contains(head, h) {
			t.Errorf("projects head %q is missing %q", head, h)
		}
	}
	if strings.Contains(head, "cost share") {
		t.Error("only the models table says cost share")
	}
	clalt := lineWith(lines, "> clalt")
	for _, want := range []string{"66%", "12.3M", "~$47.10", "40"} {
		if !strings.Contains(clalt, want) {
			t.Errorf("the clalt row %q is missing %q", clalt, want)
		}
	}
	if !strings.Contains(lineWith(lines, "web"), "13%") || !strings.Contains(lineWith(lines, "other"), "21%") {
		t.Errorf("shares are of the table total:\n%s", strings.Join(lines, "\n"))
	}
	if lineWith(lines, "Last 30 days. Share is by cost.") == "" {
		t.Error("the note line says what the share is")
	}
	if c.rows != 3 {
		t.Errorf("one cursor row per project, got %d", c.rows)
	}
}

func TestModelsTableSaysCostShare(t *testing.T) {
	lines := linesOf(historyContent(historyCtx(wideBody, 2), testAccount()))
	head := lineWith(lines, "model")
	for _, h := range []string{"cost share", "tokens", "cost"} {
		if !strings.Contains(head, h) {
			t.Errorf("models head %q is missing %q", head, h)
		}
	}
	if strings.Contains(head, "sessions") {
		t.Error("models have no sessions column")
	}
	opus := lineWith(lines, "> Opus 5.5")
	for _, want := range []string{"69%", "12.1M", "~$49.10"} {
		if !strings.Contains(opus, want) {
			t.Errorf("the opus row %q is missing %q", opus, want)
		}
	}
	if !strings.Contains(lineWith(lines, "Sonnet 5.5"), "31%") {
		t.Error("sonnet is 31 percent by cost")
	}
}

func TestShareTablesFitEveryWidth(t *testing.T) {
	for sub := 1; sub <= 2; sub++ {
		for w := 43; w <= 140; w++ {
			for i, l := range linesOf(historyContent(historyCtx(w, sub), testAccount())) {
				if len(l) > w {
					t.Fatalf("sub %d w=%d line %d is %d wide: %q", sub, w, i, len(l), l)
				}
			}
		}
	}
}

func TestHistoryCursorMarkerFollowsTheRow(t *testing.T) {
	ctx := historyCtx(wideBody, 1)
	ctx.row = 2
	out := joined(historyContent(ctx, testAccount()))
	if !strings.Contains(out, "> other") || strings.Contains(out, "> clalt") {
		t.Errorf("row 2 carries the marker:\n%s", out)
	}
}

func TestHistoryEmptyStates(t *testing.T) {
	for sub := 0; sub < 3; sub++ {
		if got := linesOf(historyContent(historyCtx(wideBody, sub), signedOutAccount())); len(got) != 1 || got[0] != "Signed out. Press s to sign in." {
			t.Errorf("signed out history tab %d = %q", sub, got)
		}
	}
	a := testAccount()
	a.History = History{}
	for sub := 0; sub < 3; sub++ {
		if out := joined(historyContent(historyCtx(wideBody, sub), a)); !strings.Contains(out, "No usage history yet.") {
			t.Errorf("tab %d without history should say so:\n%s", sub, out)
		}
	}
}
