package dashboard

import (
	"strings"
	"testing"
	"time"
)

// fiveAccounts follows the approved list: robin 26% with two sessions, morgan
// 22%, casey 95% with one waiting, jordan 5%, harriet signed out
func fiveAccounts() []Account {
	named := func(id string, pct int, sessions ...Session) Account {
		a := testAccount()
		a.ID, a.Label = id, id
		a.Limits.Five.Percent = pct
		a.Sessions = sessions
		return a
	}
	k := testAccount()
	p := named("morgan", 22)
	an := named("casey", 95, Session{ID: "a1", State: StateWaiting})
	g := named("jordan", 5)
	p.Limits.Seven.Percent, an.Limits.Seven.Percent, g.Limits.Seven.Percent = 30, 80, 11
	return []Account{k, p, an, g, signedOutAccount()}
}

func TestAccountRowsMatchTheApprovedList(t *testing.T) {
	accts := fiveAccounts()
	wide := []string{
		"> robin     26%   * 2",
		"  morgan    22%",
		"  casey     95% ! * 1",
		"  jordan     5%",
		"  harriet  signed out",
	}
	for i, want := range wide {
		row := accountRow(asciiGlyphs(), accts[i], testNow, i == 0, true, 21)
		if got := plainText(row); strings.TrimRight(got, " ") != want || row.width() != 21 {
			t.Errorf("wide row %d = %q (%d wide), want %q", i, got, row.width(), want)
		}
	}
	narrow := []string{
		"> robin      26%",
		"  morgan     22%",
		"  casey      95% !",
		"  jordan      5%",
		"  harriet no login",
	}
	for i, want := range narrow {
		row := accountRow(asciiGlyphs(), accts[i], testNow, i == 0, false, 18)
		if got := plainText(row); strings.TrimRight(got, " ") != want || row.width() != 18 {
			t.Errorf("narrow row %d = %q (%d wide), want %q", i, got, row.width(), want)
		}
	}
}

func TestNearLimitMarkerStartsAtSeventyPercent(t *testing.T) {
	for pct, mark := range map[int]bool{0: false, 69: false, 70: true, 89: true, 95: true, 100: true} {
		a := testAccount()
		a.Limits.Five.Percent = pct
		a.Sessions = nil
		got := strings.Contains(plainText(accountRow(asciiGlyphs(), a, testNow, false, true, 21)), "!")
		if got != mark {
			t.Errorf("%d%%: marker %v, want %v", pct, got, mark)
		}
	}
}

func TestAccountRowTonesCarryMeaning(t *testing.T) {
	accts := fiveAccounts()
	toneOf := func(row rich, text string) tone {
		for _, sp := range row {
			if strings.Contains(sp.s, text) {
				return sp.t
			}
		}
		return -1
	}
	if got := toneOf(accountRow(asciiGlyphs(), accts[0], testNow, true, true, 21), "robin"); got != toneBold {
		t.Errorf("the selected account is bold, got tone %d", got)
	}
	if got := toneOf(accountRow(asciiGlyphs(), accts[4], testNow, false, true, 21), "harriet"); got != toneDim {
		t.Errorf("a signed out account is dim, got tone %d", got)
	}
	if got := toneOf(accountRow(asciiGlyphs(), accts[2], testNow, false, true, 21), "95%"); got != toneBad {
		t.Errorf("95%% is red, got tone %d", got)
	}
	if got := toneOf(accountRow(asciiGlyphs(), accts[2], testNow, false, true, 21), "*"); got != toneWarn {
		t.Errorf("a waiting session makes the dot yellow, got tone %d", got)
	}
	if got := toneOf(accountRow(asciiGlyphs(), accts[0], testNow, false, true, 21), "*"); got != toneWarn {
		t.Errorf("robin has one waiting session, so a yellow dot, got tone %d", got)
	}
	working := testAccount()
	working.Sessions = working.Sessions[:1]
	if got := toneOf(accountRow(asciiGlyphs(), working, testNow, false, true, 21), "*"); got != toneGood {
		t.Errorf("only working sessions make the dot green, got tone %d", got)
	}
}

func TestSessionCountCapsAtNine(t *testing.T) {
	a := testAccount()
	a.Sessions = make([]Session, 12)
	if got := plainText(accountRow(asciiGlyphs(), a, testNow, false, true, 21)); !strings.HasSuffix(strings.TrimRight(got, " "), "* +") {
		t.Errorf("more than nine sessions show a plus, got %q", got)
	}
}

func TestRoomHint(t *testing.T) {
	accts := fiveAccounts()
	if got := roomHint(accts); got != "jordan has the most room left" {
		t.Errorf("hint = %q", got)
	}
	if roomHint(accts[:1]) != "" || roomHint(nil) != "" {
		t.Error("one account has nothing to compare with")
	}
	if roomHint([]Account{accts[0], signedOutAccount()}) != "" {
		t.Error("a signed out account does not count")
	}
	tie := []Account{accts[1], accts[1], accts[3], accts[3]}
	tie[1].Label = "second"
	if got := roomHint(tie[:2]); got != "morgan has the most room left" {
		t.Errorf("a tie goes to the first account, got %q", got)
	}
}

func TestAProblemFlagsTheRowInRedWhateverTheUsage(t *testing.T) {
	a := testAccount()
	a.Limits.Five.Percent = 10
	a.Problem = "claude not found"
	row := accountRow(asciiGlyphs(), a, testNow, false, true, 21)
	if got := plainText(row); !strings.Contains(got, " 10% !") {
		t.Errorf("a problem shows the marker even at 10%%: %q", got)
	}
	for _, sp := range row {
		if sp.s == "!" && sp.t != toneBad {
			t.Errorf("the problem marker is red, got tone %d", sp.t)
		}
	}
	a.Problem = ""
	if got := plainText(accountRow(asciiGlyphs(), a, testNow, false, true, 21)); strings.Contains(got, "!") {
		t.Errorf("no problem and 10%% needs no marker: %q", got)
	}
}

func TestAProblemOnASignedOutRowTakesTheWordsPlace(t *testing.T) {
	a := signedOutAccount()
	a.Problem = "profile folder is missing"
	for _, wide := range []bool{true, false} {
		w := rowNarrow
		if wide {
			w = rowWide
		}
		row := accountRow(asciiGlyphs(), a, testNow, false, wide, w)
		if got := plainText(row); !strings.HasPrefix(got, "  harriet") || !strings.HasSuffix(got, "! problem") || row.width() != w {
			t.Errorf("wide=%v: row = %q (%d wide)", wide, got, row.width())
		}
		if sp := row[len(row)-1]; sp.t != toneBad {
			t.Errorf("wide=%v: the word is red, got tone %d", wide, sp.t)
		}
	}
}

// both windows count, and the worse one is the account's room
func TestRoomHintRanksByTheWorseOfTheTwoWindows(t *testing.T) {
	acct := func(label string, five, seven int) Account {
		a := testAccount()
		a.Label, a.Limits.Five.Percent, a.Limits.Seven.Percent = label, five, seven
		return a
	}
	cases := []struct {
		name string
		in   []Account
		want string
	}{
		{"a week almost used never wins", []Account{acct("week", 0, 99), acct("steady", 50, 20)}, "steady has the most room left"},
		{"the week decides", []Account{acct("a", 10, 70), acct("b", 40, 40)}, "b has the most room left"},
		{"the five hours decide", []Account{acct("a", 70, 10), acct("b", 40, 40)}, "b has the most room left"},
		{"both near the limit", []Account{acct("a", 95, 50), acct("b", 50, 95)}, ""},
		{"just under the line", []Account{acct("a", 89, 10), acct("b", 100, 100)}, "a has the most room left"},
		{"on the line", []Account{acct("a", 90, 10), acct("b", 91, 91)}, ""},
		{"a tie goes to the first", []Account{acct("a", 30, 40), acct("b", 40, 30)}, "a has the most room left"},
		{"signed out accounts do not count", []Account{acct("a", 10, 10), signedOutAccount()}, ""},
	}
	for _, c := range cases {
		if got := roomHint(c.in); got != c.want {
			t.Errorf("%s: roomHint = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestALoginDueWithinThreeDaysFlagsTheRowInYellow(t *testing.T) {
	flag := func(a Account) (string, tone) {
		for _, sp := range accountRow(asciiGlyphs(), a, testNow, false, true, 21) {
			if sp.s == "!" {
				return sp.s, sp.t
			}
		}
		return "", -1
	}
	cases := []struct {
		name string
		left time.Duration
		five int
		want tone
	}{
		{"three days", 3 * 24 * time.Hour, 10, toneWarn},
		{"two days and a bit", 2*24*time.Hour + time.Hour, 10, toneWarn},
		{"an hour", time.Hour, 10, toneWarn},
		{"due now", 0, 10, toneWarn},
		{"already past", -time.Hour, 10, toneWarn},
		{"more than three days", 3*24*time.Hour + time.Hour, 10, -1},
		{"due soon and near a limit", 24 * time.Hour, 75, toneWarn},
		{"red wins when a limit is near", 24 * time.Hour, 95, toneBad},
		{"far off and near a limit", 20 * 24 * time.Hour, 95, toneBad},
	}
	for _, c := range cases {
		a := testAccount()
		a.Limits.Five.Percent = c.five
		a.Login.DueAt = testNow.Add(c.left)
		if got, tn := flag(a); tn != c.want {
			t.Errorf("%s: flag %q tone %d, want tone %d", c.name, got, tn, c.want)
		}
	}
	unknown := testAccount()
	unknown.Limits.Five.Percent = 10
	unknown.Login.DueAt = time.Time{}
	if _, tn := flag(unknown); tn != -1 {
		t.Error("an unknown due date is no reason for a flag")
	}
	out := signedOutAccount()
	out.Login.DueAt = testNow
	if got := plainText(accountRow(asciiGlyphs(), out, testNow, false, true, 21)); strings.Contains(got, "!") {
		t.Errorf("a signed out account has no login to run out: %q", got)
	}
}

func TestAProblemOutranksALoginDueSoon(t *testing.T) {
	a := testAccount()
	a.Limits.Five.Percent = 10
	a.Login.DueAt = testNow.Add(time.Hour)
	a.Problem = "claude not found"
	for _, sp := range accountRow(asciiGlyphs(), a, testNow, false, true, 21) {
		if sp.s == "!" && sp.t != toneBad {
			t.Errorf("the problem marker is red, got tone %d", sp.t)
		}
	}
}

func TestTheFlagColumnKeepsItsPlaceWhateverTheReason(t *testing.T) {
	a := testAccount()
	a.Limits.Five.Percent = 10
	plain := plainText(accountRow(asciiGlyphs(), a, testNow, false, true, 21))
	a.Login.DueAt = testNow.Add(time.Hour)
	flagged := plainText(accountRow(asciiGlyphs(), a, testNow, false, true, 21))
	if len(plain) != len(flagged) || strings.Index(flagged, "!") != strings.Index(plain, "10%")+4 {
		t.Errorf("the mark sits right after the percent: %q vs %q", plain, flagged)
	}
}
