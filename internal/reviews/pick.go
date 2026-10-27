package reviews

import "sort"

// Candidate is an account a review may run on. FiveHourUsedPct is nil when
// the 5h window is unknown.
type Candidate struct {
	ID              string
	ConfigDir       string
	FiveHourUsedPct *float64
}

// noRoomPct is the used percent from which an account is not used.
const noRoomPct = 90

// Pick chooses the account with the most 5h room. an account with a known
// window beats one without, an account that is nearly full is left out, ties
// go to the lower id.
func Pick(cands []Candidate) (Candidate, bool) {
	var ok []Candidate
	for _, c := range cands {
		if c.FiveHourUsedPct == nil || *c.FiveHourUsedPct < noRoomPct {
			ok = append(ok, c)
		}
	}
	if len(ok) == 0 {
		return Candidate{}, false
	}
	sort.SliceStable(ok, func(i, j int) bool {
		a, b := ok[i], ok[j]
		if (a.FiveHourUsedPct == nil) != (b.FiveHourUsedPct == nil) {
			return a.FiveHourUsedPct != nil
		}
		if a.FiveHourUsedPct != nil && *a.FiveHourUsedPct != *b.FiveHourUsedPct {
			return *a.FiveHourUsedPct < *b.FiveHourUsedPct
		}
		return a.ID < b.ID
	})
	return ok[0], true
}
