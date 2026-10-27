package summary

import (
	"sort"
	"time"

	"github.com/dhitalkamal/clalt/internal/usage"
)

// who answers which account a session was under at a time, from the records
// the attribution already placed. an event has no usage of its own, so it
// takes the account of the nearest record of its session.
type who struct {
	by map[string][]mark
}

type mark struct {
	at      int64
	account string
}

func newWho(byAccount map[string][]usage.Record) *who {
	w := &who{by: map[string][]mark{}}
	for account, recs := range byAccount {
		for _, r := range recs {
			if r.SessionID != "" {
				w.by[r.SessionID] = append(w.by[r.SessionID], mark{r.Timestamp.Unix(), account})
			}
		}
	}
	for _, list := range w.by {
		sort.Slice(list, func(i, j int) bool { return list[i].at < list[j].at })
	}
	return w
}

// account is Unattributed for a session no record knows.
func (w *who) account(session string, t time.Time) string {
	list := w.by[session]
	if len(list) == 0 {
		return usage.Unattributed
	}
	u := t.Unix()
	i := sort.Search(len(list), func(i int) bool { return list[i].at >= u })
	switch {
	case i == len(list):
		return list[i-1].account
	case i == 0 || list[i].at-u < u-list[i-1].at:
		return list[i].account
	}
	return list[i-1].account
}
