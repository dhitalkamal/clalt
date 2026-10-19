package dashboard

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func warned(w ...string) Snapshot {
	s := fiveSnapshot()
	s.Warnings = w
	return s
}

func TestWarningsShowUnderTheHeader(t *testing.T) {
	src := &fakeSource{snap: warned("claude 2.1.100 is older than 2.1.268, so limits are not read")}
	m := loadedModel(src, &fakeActions{}, testOptions(), 100, 30)
	lines := strings.Split(Screen(m), "\n")
	if len(lines) != 30 {
		t.Fatalf("the screen is still 30 lines: %d", len(lines))
	}
	if lines[1] != " ! claude 2.1.100 is older than 2.1.268, so limits are not read" {
		t.Errorf("the warning is the line under the header: %q", lines[1])
	}
	if !strings.Contains(lines[2], "+- Accounts") || !strings.Contains(lines[28], "+---") || !strings.Contains(lines[29], "q quit") {
		t.Errorf("the panes start below it and end one line sooner:\n%s", strings.Join(lines, "\n"))
	}
}

func TestSeveralWarningsShowTheFirstAndACount(t *testing.T) {
	src := &fakeSource{snap: warned("first one", "second one", "third one")}
	m := loadedModel(src, &fakeActions{}, testOptions(), 100, 30)
	if got := strings.Split(Screen(m), "\n")[1]; got != " ! first one (+2 more)" {
		t.Errorf("one line with a count: %q", got)
	}
}

func TestAWarningIsCutToFitAndNoWarningsTakeNoLine(t *testing.T) {
	long := strings.Repeat("a very long warning ", 10)
	m := loadedModel(&fakeSource{snap: warned(long)}, &fakeActions{}, testOptions(), 60, 20)
	if got := strings.Split(Screen(m), "\n")[1]; len(got) > 60 || !strings.HasSuffix(got, "...") {
		t.Errorf("a long warning ends in an ellipsis: %q", got)
	}
	clear, _ := upd(m, loadedMsg{snap: fiveSnapshot()})
	if got := strings.Split(Screen(clear), "\n")[1]; !strings.Contains(got, "+- Accounts") {
		t.Errorf("the next load without warnings gives the line back: %q", got)
	}
	blank := loadedModel(&fakeSource{snap: warned("", "  ")}, &fakeActions{}, testOptions(), 100, 30)
	if got := strings.Split(Screen(blank), "\n")[1]; !strings.Contains(got, "+- Accounts") {
		t.Errorf("empty warnings are not shown: %q", got)
	}
}

func TestWarningsDoNotMoveTheFilterCursorOffItsRow(t *testing.T) {
	m := loadedModel(&fakeSource{snap: warned("careful")}, &fakeActions{}, testOptions(), 100, 30)
	next, _ := Press(m, "/")
	if c := next.View().Cursor; c == nil || c.Position.Y != 3 {
		t.Errorf("the filter row sits one line lower with a warning: %+v", c)
	}
}

func TestALoadWithAnEmptyAccountIDIsRejected(t *testing.T) {
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, testOptions(), 100, 30)
	bad := fiveSnapshot()
	bad.Accounts[1].ID = ""
	next, _ := upd(m, loadedMsg{snap: bad})
	out := Screen(next)
	if next.loadErr == nil || !strings.Contains(out, `update failed: account 2 ("morgan") has no id`) {
		t.Errorf("the header says which account has no id:\n%s", out)
	}
	if !strings.Contains(out, "morgan") || len(next.snap.Accounts) != 5 {
		t.Error("the last good data stays")
	}
}

func TestALoadWithDuplicateAccountIDsIsRejected(t *testing.T) {
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, testOptions(), 100, 30)
	bad := fiveSnapshot()
	bad.Accounts[2].ID = bad.Accounts[0].ID
	next, _ := upd(m, loadedMsg{snap: bad})
	if out := Screen(next); !strings.Contains(out, `update failed: accounts 1 and 3 share the id "a1"`) {
		t.Errorf("the header names both accounts:\n%s", out)
	}
}

func TestABadFirstLoadFillsThePaneWithTheReason(t *testing.T) {
	m := newModel(context.Background(), &fakeSource{}, &fakeActions{}, testOptions())
	m, _ = upd(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	bad := fiveSnapshot()
	bad.Accounts[1].ID = bad.Accounts[0].ID
	m, _ = upd(m, loadedMsg{snap: bad})
	out := Screen(m)
	if m.have || !strings.Contains(out, `Could not load accounts: accounts 1 and 2 share the id "a1"`) {
		t.Errorf("no good snapshot yet, so the pane explains:\n%s", out)
	}
}

func TestAnIDWithControlCharactersIsQuotedInTheError(t *testing.T) {
	m := loadedModel(&fakeSource{snap: fiveSnapshot()}, &fakeActions{}, testOptions(), 100, 30)
	bad := fiveSnapshot()
	bad.Accounts[0].ID = "a\x1b[31m"
	bad.Accounts[1].ID = "a\x1b[31m"
	next, _ := upd(m, loadedMsg{snap: bad})
	if raw := Raw(next); strings.Contains(raw, "\x1b[31m") {
		t.Errorf("the escape sequence in an id must not reach the terminal:\n%q", raw)
	}
}
