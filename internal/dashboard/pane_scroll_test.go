package dashboard

import (
	"fmt"
	"strings"
	"testing"
)

// manyAccounts is n signed in accounts named acct01, acct02 and so on
func manyAccounts(n int) Snapshot {
	snap := Snapshot{}
	for i := 1; i <= n; i++ {
		a := testAccount()
		a.ID, a.Label = fmt.Sprintf("id%02d", i), fmt.Sprintf("acct%02d", i)
		a.Limits.Five.Percent = 10
		a.Sessions = nil
		snap.Accounts = append(snap.Accounts, a)
	}
	return snap
}

// listLines are the account rows of the left pane, without the borders
func listLines(m model) []string {
	var out []string
	for _, l := range strings.Split(Screen(m), "\n")[2 : m.h-2] {
		if len(l) > 26 {
			out = append(out, strings.TrimSpace(l[2:26]))
		}
	}
	return out
}

func TestAListLongerThanThePaneScrollsWithTheSelection(t *testing.T) {
	m := loadedModel(&fakeSource{snap: manyAccounts(30)}, &fakeActions{}, testOptions(), 100, 30)
	rows := listLines(m)
	if !strings.HasPrefix(rows[0], "> acct01") {
		t.Fatalf("setup: the first row is selected:\n%s", strings.Join(rows, "\n"))
	}
	if got := rows[len(rows)-2]; !strings.Contains(got, "acct25") || strings.TrimSpace(rows[len(rows)-1]) != "+5 more" {
		t.Errorf("25 rows and a count of the rest: %q then %q", got, rows[len(rows)-1])
	}
	for i := 1; i < 30; i++ {
		m = pressSettle(m, "j")
		out := Screen(m)
		if want := fmt.Sprintf("> acct%02d", i+1); !strings.Contains(out, want) {
			t.Fatalf("after %d presses the selected account must be on screen (%q):\n%s", i, want, out)
		}
		if !strings.Contains(out, "+5 more") {
			t.Fatalf("after %d presses the count of hidden rows stays at 5:\n%s", i, out)
		}
	}
	if out := Screen(m); strings.Contains(out, "acct01 ") || !strings.Contains(out, "acct30") || !strings.Contains(detailTitle(out), "acct30") {
		t.Errorf("at the end the window shows the last accounts:\n%s", out)
	}
	for i := 29; i > 0; i-- {
		m = pressSettle(m, "k")
		if want := fmt.Sprintf("> acct%02d", i); !strings.Contains(Screen(m), want) {
			t.Fatalf("going back up, %q must be on screen:\n%s", want, Screen(m))
		}
	}
	if out := Screen(m); !strings.Contains(out, "acct01") || m.listOff != 0 {
		t.Errorf("back at the top the window is at the top: offset %d", m.listOff)
	}
}

func TestThirteenAccountsOnTheSmallestScreenScroll(t *testing.T) {
	m := loadedModel(&fakeSource{snap: manyAccounts(13)}, &fakeActions{}, testOptions(), 60, 16)
	out := Screen(m)
	if lines := strings.Split(out, "\n"); len(lines) != 16 || !strings.Contains(out, "+2 more") || !strings.Contains(out, "acct11") || strings.Contains(out, "acct12 ") {
		t.Fatalf("eleven rows and a count of two:\n%s", out)
	}
	for i := 1; i < 13; i++ {
		m = pressSettle(m, "j")
	}
	out = Screen(m)
	if !strings.Contains(out, "> acct13") || !strings.Contains(out, "+2 more") || strings.Contains(out, "acct01 ") {
		t.Errorf("the last account is selected and on screen:\n%s", out)
	}
	if len(strings.Split(out, "\n")) != 16 {
		t.Error("the screen keeps its height")
	}
}

func TestAListThatFitsHasNoCountAndKeepsTheRoomHint(t *testing.T) {
	m := loadedModel(&fakeSource{snap: manyAccounts(5)}, &fakeActions{}, testOptions(), 100, 30)
	if out := Screen(m); strings.Contains(out, "more") && !strings.Contains(out, "has the most") {
		t.Errorf("five accounts all show:\n%s", out)
	}
	exact := loadedModel(&fakeSource{snap: manyAccounts(26)}, &fakeActions{}, testOptions(), 100, 30)
	if out := Screen(exact); strings.Contains(out, "more") || !strings.Contains(out, "acct26") {
		t.Errorf("26 rows fill the pane exactly, so there is no count:\n%s", out)
	}
	over := loadedModel(&fakeSource{snap: manyAccounts(27)}, &fakeActions{}, testOptions(), 100, 30)
	if out := Screen(over); !strings.Contains(out, "+2 more") || strings.Contains(out, "acct26") {
		t.Errorf("27 rows leave 25 and a count of 2:\n%s", out)
	}
}

func TestTheFilterTakesTwoLinesFromTheList(t *testing.T) {
	m := loadedModel(&fakeSource{snap: manyAccounts(30)}, &fakeActions{}, testOptions(), 100, 30)
	m = pressSettle(m, "/", "a", "c", "c", "t")
	out := Screen(m)
	if !strings.Contains(out, "+7 more") || !strings.Contains(out, "acct23 ") || strings.Contains(out, "acct24 ") {
		t.Errorf("23 rows, the filter and a blank line, and a count of 7:\n%s", out)
	}
	narrowed := pressSettle(m, "0", "1")
	if out := Screen(narrowed); strings.Contains(out, "more") || !strings.Contains(out, "> acct01") {
		t.Errorf("a narrower filter needs no window:\n%s", out)
	}
}

func TestTheWindowFollowsAClampedOffsetWhenTheListShrinks(t *testing.T) {
	src := &fakeSource{snap: manyAccounts(30)}
	m := loadedModel(src, &fakeActions{}, testOptions(), 100, 30)
	for i := 0; i < 29; i++ {
		m = pressSettle(m, "j")
	}
	if m.listOff == 0 {
		t.Fatal("setup: the window moved")
	}
	m, _ = upd(m, loadedMsg{snap: manyAccounts(10)})
	if m.listOff != 0 {
		t.Errorf("ten accounts all fit, so the offset is back at 0: %d", m.listOff)
	}
	if out := Screen(m); strings.Contains(out, "more") || !strings.Contains(out, "acct10") {
		t.Errorf("all ten show:\n%s", out)
	}
}
