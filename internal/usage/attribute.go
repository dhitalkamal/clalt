package usage

import (
	"sort"
	"time"
)

// Unattributed is the bucket for usage that cannot be tied to an account: it
// predates any recorded switch, or it is newer than the switch log, which
// stopped growing when accounts became folders of their own (see
// AttributeUntil). Reports use it as a label, so login and rename refuse it as
// the label of an account.
const Unattributed = "unattributed"

// SwitchEvent mirrors account.SwitchEvent without importing that package, so the
// usage package stays decoupled. The cli layer converts between them.
type SwitchEvent struct {
	Ts      time.Time
	Account string
}

// Attribute groups records by the account that was active at each record's time,
// per the switch log. A record is owned by the most recent switch at or before
// its timestamp; records before the first switch go to Unattributed.
func Attribute(records []Record, events []SwitchEvent) map[string][]Record {
	return AttributeUntil(records, events, time.Time{})
}

// AttributeUntil is Attribute for a log that stopped being written at until.
// from then on nothing says which account a record belongs to, and it would be
// wrong to give it to the account of the last event, so records at or after until
// go to Unattributed. a zero until means the log is all there is.
func AttributeUntil(records []Record, events []SwitchEvent, until time.Time) map[string][]Record {
	sorted := make([]SwitchEvent, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Ts.Before(sorted[j].Ts) })

	out := map[string][]Record{}
	for _, r := range records {
		account := Unattributed
		if until.IsZero() || r.Timestamp.Before(until) {
			for _, e := range sorted {
				if e.Ts.After(r.Timestamp) {
					break
				}
				account = e.Account
			}
		}
		out[account] = append(out[account], r)
	}
	return out
}
