package usage

import (
	"sort"
	"time"
)

// Unattributed is the bucket for usage that predates any recorded switch, so it
// cannot be tied to an account. Documented limitation of local attribution.
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
	sorted := make([]SwitchEvent, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Ts.Before(sorted[j].Ts) })

	out := map[string][]Record{}
	for _, r := range records {
		account := Unattributed
		for _, e := range sorted {
			if e.Ts.After(r.Timestamp) {
				break
			}
			account = e.Account
		}
		out[account] = append(out[account], r)
	}
	return out
}
