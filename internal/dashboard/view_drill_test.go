package dashboard

import (
	"fmt"
	"strings"
	"testing"
)

func drillCtx(w int, project string) viewCtx {
	c := historyCtx(w, 1)
	c.drill = project
	return c
}

// drillRow is one table line: a two cell margin, then the five columns
func drillRow(day, model, sessions, tokens, cost string) string {
	return fmt.Sprintf("  %-6s  %-10s  %8s  %6s  %6s", day, model, sessions, tokens, cost)
}

func TestDrillShowsTheBreadcrumbTotalsAndTable(t *testing.T) {
	c := historyContent(drillCtx(wideBody, "clalt"), testAccount())
	lines := linesOf(c)
	want := []string{
		"Projects / clalt",
		"Total   ~$47.10 (66% of cost)   12.3M tokens   40 sessions",
		"",
		drillRow("day", "model", "sessions", "tokens", "cost"),
		drillRow("Wed 15", "Opus 5.5", "2", "340k", "~$1.52"),
		drillRow("Tue 14", "Opus 5.5", "3", "900k", "~$3.10"),
		drillRow("Tue 14", "Sonnet 5.5", "1", "210k", "~$0.52"),
		drillRow("Mon 13", "Sonnet 5.5", "3", "1.3M", "~$2.87"),
		drillRow("Fri 10", "Opus 5.5", "5", "1.6M", "~$4.05"),
		drillRow("Thu 9", "Opus 5.5", "4", "12k", "~$0.03"),
		"",
		"Last 30 days, newest first.",
	}
	if len(lines) != len(want) {
		t.Fatalf("%d lines, want %d:\n%s", len(lines), len(want), strings.Join(lines, "\n"))
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestDrillKeepsTheTableHeaderFixedAndScrollsTheRows(t *testing.T) {
	c := historyContent(drillCtx(wideBody, "clalt"), testAccount())
	if len(c.head) != 4 || plainText(c.head[3]) != drillRow("day", "model", "sessions", "tokens", "cost") {
		t.Errorf("the head is the breadcrumb, the totals, a blank and the table header: %q", linesOf(content{head: c.head}))
	}
	if c.rows != 0 {
		t.Errorf("rows are only scrolled, there is no cursor: rows = %d", c.rows)
	}
	if len(c.body) != 6+2 {
		t.Errorf("six rows, a blank line and the note: %d body lines", len(c.body))
	}
}

func TestDrillHasNeitherSubTabsNorACursor(t *testing.T) {
	out := joined(historyContent(drillCtx(wideBody, "clalt"), testAccount()))
	for _, gone := range []string{"[Projects]", "Days", "Models", "> "} {
		if strings.Contains(out, gone) {
			t.Errorf("the breadcrumb replaces the sub tabs and nothing is selectable, found %q in\n%s", gone, out)
		}
	}
}

func TestDrillBreadcrumbTones(t *testing.T) {
	c := historyContent(drillCtx(wideBody, "clalt"), testAccount())
	if sp := spanContaining(c, "Projects"); sp.t != toneDim {
		t.Errorf("the trail is dim, got tone %d", sp.t)
	}
	if sp := spanContaining(c, "clalt"); sp.t != toneBold {
		t.Errorf("the project name is bold, got tone %d", sp.t)
	}
}

func TestDrillFollowsTheNamedProject(t *testing.T) {
	lines := linesOf(historyContent(drillCtx(wideBody, "web"), testAccount()))
	if lines[0] != "Projects / web" || !strings.Contains(lines[1], "~$9.26 (13% of cost)") || !strings.Contains(lines[1], "20 sessions") {
		t.Errorf("web totals = %q / %q", lines[0], lines[1])
	}
	if lineWith(lines, drillRow("Sun 12", "Sonnet 5.5", "1", "120k", "~$0.95")) == "" {
		t.Errorf("web has its own row:\n%s", strings.Join(lines, "\n"))
	}
	if strings.Contains(strings.Join(lines, "\n"), "Wed 15") {
		t.Error("another project's rows must not show")
	}
}

func TestDrillSingularSession(t *testing.T) {
	a := testAccount()
	a.History.Projects[0].Sessions = 1
	if lines := linesOf(historyContent(drillCtx(wideBody, "clalt"), a)); !strings.HasSuffix(lines[1], "1 session") {
		t.Errorf("one session is singular: %q", lines[1])
	}
}

func TestDrillWithoutActivitySaysSo(t *testing.T) {
	c := historyContent(drillCtx(wideBody, "other"), testAccount())
	lines := linesOf(c)
	if lines[0] != "Projects / other" || !strings.Contains(joined(c), "No activity recorded for this project.") {
		t.Errorf("a project without rows still shows its totals and says so:\n%s", strings.Join(lines, "\n"))
	}
	if strings.Contains(joined(c), "sessions  tokens") {
		t.Error("no table header without rows")
	}
}

func TestDrillOfAProjectThatIsGoneShowsTheTable(t *testing.T) {
	out := joined(historyContent(drillCtx(wideBody, "ghost"), testAccount()))
	if !strings.Contains(out, "[Projects]") || !strings.Contains(out, "> clalt") {
		t.Errorf("an unknown project falls back to the projects table:\n%s", out)
	}
}

func TestDrillOnlyAppliesToTheProjectsTab(t *testing.T) {
	c := historyCtx(wideBody, 2)
	c.drill = "clalt"
	if out := joined(historyContent(c, testAccount())); !strings.Contains(out, "[Models]") {
		t.Errorf("the models tab never drills in:\n%s", out)
	}
}

func TestDrillSignedOutStillShowsThePrompt(t *testing.T) {
	if got := linesOf(historyContent(drillCtx(wideBody, "clalt"), signedOutAccount())); len(got) != 1 || got[0] != "Signed out. Press s to sign in." {
		t.Errorf("signed out drill = %q", got)
	}
}

func TestDrillFitsEveryWidth(t *testing.T) {
	for w := 43; w <= 140; w++ {
		for i, l := range linesOf(historyContent(drillCtx(w, "clalt"), testAccount())) {
			if len(l) > w {
				t.Fatalf("w=%d line %d is %d wide: %q", w, i, len(l), l)
			}
		}
	}
}

func TestNarrowDrillStacksTheTotalsAndKeepsTheTableReadable(t *testing.T) {
	c := historyContent(drillCtx(43, "clalt"), testAccount())
	lines := linesOf(c)
	if len(c.head) != 5 {
		t.Fatalf("the totals take two lines at 43 columns, so the head has 5 lines: %q", lines)
	}
	if lines[1] != "Total   ~$47.10 (66% of cost)" || lines[2] != "        12.3M tokens   40 sessions" {
		t.Errorf("the cost comes first, then the usage, and a number never parts from its unit:\n%s", strings.Join(lines[:3], "\n"))
	}
	header := lines[len(c.head)-1]
	for _, h := range []string{"day", "model", "sessions", "tokens", "cost"} {
		if !strings.Contains(header, h) {
			t.Errorf("header %q is missing %q", header, h)
		}
	}
	for _, d := range []string{"Wed 15", "Thu 9"} {
		if lineWith(lines, d) == "" {
			t.Errorf("row %s is missing", d)
		}
	}
}

func TestDrillTotalsStillFitAtTwentyFourColumns(t *testing.T) {
	lines := linesOf(historyContent(drillCtx(24, "clalt"), testAccount()))
	for i, l := range lines[:5] {
		if len(l) > 24 {
			t.Errorf("line %d is %d wide at 24 columns: %q", i, len(l), l)
		}
	}
	if !strings.Contains(strings.Join(lines[:6], "\n"), "40 sessions") {
		t.Errorf("every total survives the wrap:\n%s", strings.Join(lines[:6], "\n"))
	}
}

func TestDrillBreadcrumbIsCutToFit(t *testing.T) {
	a := testAccount()
	a.History.Projects[0].Name = strings.Repeat("long-project-name-", 5)
	first := linesOf(historyContent(drillCtx(43, a.History.Projects[0].Name), a))[0]
	if len(first) > 43 || !strings.HasSuffix(first, "...") || !strings.HasPrefix(first, "Projects / long") {
		t.Errorf("a long name is cut with an ellipsis: %q", first)
	}
}

func TestDrillUsesTheEllipsisGlyphInUnicodeMode(t *testing.T) {
	a := testAccount()
	a.History.Projects[0].Name = strings.Repeat("long-project-name-", 5)
	c := drillCtx(43, a.History.Projects[0].Name)
	c.g = unicodeGlyphs()
	if first := linesOf(historyContent(c, a))[0]; !strings.HasSuffix(first, cp(0x2026)) {
		t.Errorf("unicode mode cuts with the real ellipsis: %q", first)
	}
}
