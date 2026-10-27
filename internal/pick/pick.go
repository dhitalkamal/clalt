package pick

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/rules"
)

// Candidate is an account as the picker sees it.
type Candidate struct {
	ID, Label  string
	Hidden     bool
	SignedIn   bool // a sign-in is recorded for it
	Folders    []string
	AutoSwitch bool
}

// Options say what the choice is for.
type Options struct {
	Cwd string
	Now time.Time
	// Limits looks up the limits of an account by id.
	Limits func(id string) (Limits, bool)
	// MaxAge is how old limit data may be, DefaultMaxAge when zero.
	MaxAge time.Duration
	// Exclude is an account that is never a candidate: the one a session is
	// leaving.
	Exclude string
	// NeedAutoSwitch leaves out the accounts that auto-switch may not use.
	NeedAutoSwitch bool
}

// Ranked is an account that may be used, with how it compares. Reason is set on
// the first one only: why it was picked.
type Ranked struct {
	Candidate
	Room    float64
	HasData bool
	Reason  string
}

// Left is an account that may not be used, and why.
type Left struct {
	Candidate
	Why string
}

// Result is the accounts that may be used, best first, and those that may not.
type Result struct {
	Ranked []Ranked
	Left   []Left
}

// Rank orders the accounts that may serve Cwd. the ones with recent limit data
// come first, most room first. the ones without come after them, and ties go by
// label. an account that is removed, not signed in, excluded, kept out by
// auto-switch or kept out of this folder by its rules is left out, with the
// reason.
func Rank(cands []Candidate, o Options) Result {
	maxAge := o.MaxAge
	if maxAge == 0 {
		maxAge = DefaultMaxAge
	}
	var res Result
	used := map[string]string{}
	for _, c := range cands {
		if why := leftOut(c, o); why != "" {
			res.Left = append(res.Left, Left{Candidate: c, Why: why})
			continue
		}
		r := Ranked{Candidate: c}
		if o.Limits != nil {
			if l, ok := o.Limits(c.ID); ok && l.Recent(o.Now, maxAge) {
				if room, known := l.Room(o.Now); known {
					r.Room, r.HasData = room, true
					used[c.ID] = usedText(l, o.Now)
				}
			}
		}
		res.Ranked = append(res.Ranked, r)
	}
	sort.SliceStable(res.Ranked, func(i, j int) bool { return before(res.Ranked[i], res.Ranked[j]) })
	if len(res.Ranked) > 0 {
		first := &res.Ranked[0]
		switch {
		case len(res.Ranked) == 1:
			first.Reason = "the only allowed account"
		case first.HasData:
			first.Reason = "most room (" + used[first.ID] + ")"
		default:
			first.Reason = "no recent limit data, first by label"
		}
	}
	return res
}

func leftOut(c Candidate, o Options) string {
	switch {
	case c.Hidden:
		return "was removed"
	case o.Exclude != "" && c.ID == o.Exclude:
		return "is the account the session is in"
	case !c.SignedIn:
		return "is not signed in"
	case o.NeedAutoSwitch && !c.AutoSwitch:
		return "has auto-switch off"
	case !rules.Allows(c.Folders, o.Cwd):
		return "its folder rules do not include " + o.Cwd
	}
	return ""
}

func before(a, b Ranked) bool {
	if a.HasData != b.HasData {
		return a.HasData
	}
	if a.HasData && a.Room != b.Room {
		return a.Room > b.Room
	}
	if la, lb := strings.ToLower(a.Label), strings.ToLower(b.Label); la != lb {
		return la < lb
	}
	if a.Label != b.Label {
		return a.Label < b.Label
	}
	return a.ID < b.ID
}

// usedText says how much of each known window is used.
func usedText(l Limits, now time.Time) string {
	var parts []string
	if u, ok := l.FiveHour.used(now); ok {
		parts = append(parts, fmt.Sprintf("5-hour window %.0f%% used", u))
	}
	if u, ok := l.SevenDay.used(now); ok {
		parts = append(parts, fmt.Sprintf("7-day window %.0f%% used", u))
	}
	return strings.Join(parts, ", ")
}
