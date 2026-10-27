package dashboard

import (
	"strings"
	"testing"
)

// a feed can know one window and not the other: the unknown one is not 0%
func halfKnown(a Account, five, seven bool) Account {
	a.Limits.Five.Unknown, a.Limits.Seven.Unknown = five, seven
	return a
}

func TestAnUnknownWindowGetsNoBarAndSaysSo(t *testing.T) {
	a := halfKnown(testAccount(), false, true)
	out := strings.Join(linesOf(overviewContent(ctxFor(wideBody), a)), "\n")
	var seven string
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "7d") {
			seven = l
		}
	}
	if !strings.Contains(seven, "not reported") || strings.Contains(seven, "%") {
		t.Errorf("7d line = %q", seven)
	}
	if !strings.Contains(out, "5h") {
		t.Errorf("the known window should still be drawn:\n%s", out)
	}
}

func TestAnUnknownFiveHourWindowShowsDashesInTheList(t *testing.T) {
	a := halfKnown(testAccount(), true, false)
	a.Label, a.Sessions = "jordan", nil
	row := plainText(accountRow(asciiGlyphs(), a, testNow, false, true, rowWide))
	if !strings.Contains(row, "--") || strings.Contains(row, "%") {
		t.Errorf("row = %q", row)
	}
}

func TestAnAccountWithAnUnknownWindowIsNotRankedForRoom(t *testing.T) {
	mk := func(label string, used int, five, seven bool) Account {
		a := testAccount()
		a.Label, a.Limits.Five.Percent, a.Limits.Seven.Percent = label, used, used
		return halfKnown(a, five, seven)
	}
	got := roomHint([]Account{mk("a", 40, false, false), mk("b", 0, false, true), mk("c", 60, false, false)})
	if got != "a has the most room left" {
		t.Errorf("hint = %q", got)
	}
}
