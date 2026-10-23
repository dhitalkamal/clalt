package dashboard

import (
	"fmt"
	"time"
)

const (
	rowWide   = 21 // account row width in the wide layout
	rowNarrow = 18 // and in the narrow layout, which drops the session dots
)

func countText(n int) string {
	if n > 9 {
		return "+"
	}
	return fmt.Sprint(n)
}

// dotTone is yellow while any session waits for the user
func dotTone(a Account) tone {
	for _, s := range a.Sessions {
		if s.State == StateWaiting {
			return toneWarn
		}
	}
	return toneGood
}

// loginSoonDays is how close the 30 day cap flags an account in the list
const loginSoonDays = 3

// rowFlag is the mark after the percent. a problem is red, a limit is red from
// 90 and yellow from 70, and a login due within three days is yellow, so red
// wins when an account has both.
func rowFlag(a Account, now time.Time) span {
	pct := a.Limits.Five.Percent
	switch {
	case a.Problem != "":
		return bad("!")
	case pct >= 70:
		return span{pctTone(pct), "!"}
	case !a.Login.DueAt.IsZero() && daysLeft(a.Login.DueAt, now) <= loginSoonDays:
		return warn("!")
	}
	return gap(1)
}

// accountRow draws one list row. the right cluster (5h percent, warning mark,
// session dot and count) has a fixed width so the columns line up.
func accountRow(g glyphs, a Account, now time.Time, selected, wide bool, w int) rich {
	mark, nameTone := " ", toneText
	if selected {
		mark, nameTone = g.cursor, toneBold
	} else if !a.SignedIn {
		nameTone = toneDim
	}
	left := rich{span{nameTone, mark}, gap(1), span{nameTone, a.Label}}
	if !a.SignedIn {
		word := rich{dim("signed out")}
		if !wide {
			word = rich{dim("no login")}
		}
		if a.Problem != "" {
			word = rich{bad("! problem")}
		}
		return alignRight(left, word, w)
	}
	pct := a.Limits.Five.Percent
	cell := span{pctTone(pct), fmt.Sprintf("%3d%%", pct)}
	if !a.Limits.known() {
		cell = dim("  --")
	}
	right := rich{cell, gap(1), rowFlag(a, now)}
	clusterW := 6
	if wide {
		clusterW = 10
		if n := len(a.Sessions); n > 0 {
			right = append(right, gap(1), span{dotTone(a), g.dot}, gap(1), dim(countText(n)))
		}
	}
	return alignRight(left, right.fit(clusterW), w)
}

// roomLimit is the usage from which an account has no room left to name
const roomLimit = 90

// worstWindow is the higher of the two usages, which is what limits an account
func worstWindow(a Account) int { return max(a.Limits.Five.Percent, a.Limits.Seven.Percent) }

// roomHint names the signed in account with the most room left, the one whose
// worse window has the lowest usage. an account at 90 or more in either window
// has no room to name, and one with no limit reading has no usage to compare.
// with fewer than two accounts to compare there is nothing to name.
func roomHint(accts []Account) string {
	best, signedIn := -1, 0
	for i, a := range accts {
		if !a.SignedIn || !a.Limits.known() {
			continue
		}
		signedIn++
		if worst := worstWindow(a); worst < roomLimit && (best < 0 || worst < worstWindow(accts[best])) {
			best = i
		}
	}
	if signedIn < 2 || best < 0 {
		return ""
	}
	return accts[best].Label + " has the most room left"
}
