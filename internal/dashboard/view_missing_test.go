package dashboard

import (
	"strings"
	"testing"
)

// until the feed brings a reading a limit is not known, and the views must not
// show it as 0%. a source says so by leaving Limits empty.
func noReading(a Account) Account {
	a.Limits = Limits{}
	return a
}

func TestAnAccountWithNoLimitReadingShowsDashesNotZero(t *testing.T) {
	a := noReading(testAccount())
	a.Label, a.Sessions = "jordan", nil
	for _, c := range []struct {
		wide bool
		w    int
		want string
	}{
		{true, rowWide, "  jordan     --"},
		{false, rowNarrow, "  jordan      --"},
	} {
		row := accountRow(asciiGlyphs(), a, testNow, false, c.wide, c.w)
		if got := strings.TrimRight(plainText(row), " "); got != c.want || row.width() != c.w {
			t.Errorf("wide=%v: row = %q (%d wide), want %q", c.wide, got, row.width(), c.want)
		}
		var dashes span
		for _, sp := range row {
			if sp.s == "  --" {
				dashes = sp
			}
		}
		if dashes.t != toneDim {
			t.Errorf("wide=%v: the dashes are dim, got tone %d", c.wide, dashes.t)
		}
	}
}

func TestTheOverviewDrawsNoBarsWithoutALimitReading(t *testing.T) {
	lines := linesOf(overviewContent(ctxFor(wideBody), noReading(testAccount())))
	for _, l := range lines {
		if strings.HasPrefix(l, "5h") || strings.HasPrefix(l, "7d") {
			t.Errorf("a bar for a limit that nobody has read: %q", l)
		}
	}
	out := strings.Join(lines, "\n")
	for _, want := range []string{"No limit reading yet.", "Budget", "Today", "Week", "Running"} {
		if !strings.Contains(out, want) {
			t.Errorf("the overview is missing %q:\n%s", want, out)
		}
	}
}

func TestAnAccountWithNoReadingIsNotRankedForRoom(t *testing.T) {
	known := func(label string, used int) Account {
		a := testAccount()
		a.Label, a.Limits.Five.Percent, a.Limits.Seven.Percent = label, used, used
		return a
	}
	fresh := noReading(testAccount())
	fresh.Label = "fresh"
	cases := []struct {
		name string
		in   []Account
		want string
	}{
		{"an unknown account never wins", []Account{fresh, known("a", 40), known("b", 60)}, "a has the most room left"},
		{"one reading is nothing to compare", []Account{fresh, known("a", 40)}, ""},
		{"readings still compare with each other", []Account{known("a", 60), fresh, known("b", 40)}, "b has the most room left"},
	}
	for _, c := range cases {
		if got := roomHint(c.in); got != c.want {
			t.Errorf("%s: roomHint = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestASessionWithNoLinesChangedLeavesThemOut(t *testing.T) {
	s := testAccount().Sessions[0]
	s.Added, s.Removed = 0, 0
	rows := sessionRows(ctxFor(wideBody), s, false)
	if got := plainText(rows[2]); strings.Contains(got, "lines") {
		t.Errorf("nothing is known of the lines, so none are shown: %q", got)
	}
	s.Added = 5
	rows = sessionRows(ctxFor(wideBody), s, false)
	if got := plainText(rows[2]); !strings.Contains(got, "+5 -0 lines") {
		t.Errorf("lines that changed are shown: %q", got)
	}
}

func TestTheProfileLeavesOutTheFolderOfALastUseThatIsUnknown(t *testing.T) {
	a := testAccount()
	a.LastIn = ""
	got := lineWith(linesOf(profileContent(ctxFor(wideBody), a)), "Last used")
	if strings.TrimRight(got, " ") != "Last used  4m ago" {
		t.Errorf("the last use without a folder: %q", got)
	}
	a.LastIn = "~/Projects/personal/clalt"
	got = lineWith(linesOf(profileContent(ctxFor(wideBody), a)), "Last used")
	if !strings.HasSuffix(strings.TrimRight(got, " "), "4m ago in ~/Projects/personal/clalt") {
		t.Errorf("the last use with a folder: %q", got)
	}
}
