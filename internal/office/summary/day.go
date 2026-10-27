package summary

import (
	"sort"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/usage"
)

const maxName = 60

// dayBuilder holds the accumulators of one summary.
type dayBuilder struct {
	s        *scan
	who      *who
	accounts map[string]*acc
	projects map[[2]string]*acc
	total    *acc
}

func (b *dayBuilder) at(account, project string) (*acc, *acc) {
	a := b.accounts[account]
	if a == nil {
		a = newAcc()
		b.accounts[account] = a
	}
	k := [2]string{account, project}
	p := b.projects[k]
	if p == nil {
		p = newAcc()
		b.projects[k] = p
	}
	return a, p
}

// project is the folder the session worked in, else the folder claude filed it under.
func (b *dayBuilder) project(session string) string {
	if c := b.s.cwd[session]; c != "" {
		if n := clean(lastFolder(c), maxName); n != "" {
			return n
		}
	}
	if d := clean(b.s.dir[session], maxName); d != "" {
		return d
	}
	return "unknown"
}

// buildDay turns a scan of one local day into the summary.
func buildDay(s *scan, byAccount map[string][]usage.Record, w window, order []Account) Summary {
	b := &dayBuilder{s: s, who: newWho(byAccount), accounts: map[string]*acc{}, projects: map[[2]string]*acc{}, total: newAcc()}
	for account, recs := range byAccount {
		for _, r := range recs {
			if !w.has(r.Timestamp) {
				continue
			}
			a, p := b.at(account, b.project(r.SessionID))
			cost := usage.CostOf(r)
			for _, x := range []*acc{a, p, b.total} {
				x.tokens += r.Billable()
				x.cost += cost
			}
		}
	}
	for _, e := range s.events {
		account := b.who.account(e.session, e.at)
		a, p := b.at(account, b.project(e.session))
		for _, x := range []*acc{a, p, b.total} {
			x.event(e)
		}
	}
	for id, stamps := range s.stamps {
		sort.Slice(stamps, func(i, j int) bool { return stamps[i] < stamps[j] })
		proj := b.project(id)
		for _, t := range []int64{stamps[0], stamps[len(stamps)-1]} {
			a, p := b.at(b.who.account(id, time.Unix(t, 0)), proj)
			a.sessions[id], p.sessions[id] = true, true
		}
		b.total.sessions[id] = true
		for i := 1; i < len(stamps); i++ {
			if gap := stamps[i] - stamps[i-1]; gap > 0 && gap <= idleGap {
				a, p := b.at(b.who.account(id, time.Unix(stamps[i], 0)), proj)
				a.activeSec += gap
				p.activeSec += gap
				b.total.activeSec += gap
			}
		}
	}
	return b.out(w, order)
}

func (b *dayBuilder) out(w window, order []Account) Summary {
	sum := Summary{V: Version, Day: w.from.Format("2006-01-02"), From: w.from.Format(time.RFC3339), To: w.to.Format(time.RFC3339),
		Totals: b.total.stats(), Accounts: []AccountOut{}}
	for _, id := range accountOrder(order, keys(b.accounts)) {
		a := AccountOut{ID: id, Label: labelOf(order, id), Stats: b.accounts[id].stats(), Projects: []ProjectOut{}}
		for k, p := range b.projects {
			if k[0] == id {
				a.Projects = append(a.Projects, ProjectOut{Name: k[1], Stats: p.stats()})
			}
		}
		sort.Slice(a.Projects, func(i, j int) bool {
			x, y := a.Projects[i], a.Projects[j]
			if x.Tokens != y.Tokens {
				return x.Tokens > y.Tokens
			}
			return x.Name < y.Name
		})
		sum.Accounts = append(sum.Accounts, a)
	}
	return sum
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// accountOrder is the known accounts in their order, then ids nobody listed
// by name, with unattributed last.
func accountOrder(order []Account, have []string) []string {
	in := map[string]bool{}
	for _, id := range have {
		in[id] = true
	}
	var out, rest []string
	for _, a := range order {
		if in[a.ID] {
			out = append(out, a.ID)
			delete(in, a.ID)
		}
	}
	for id := range in {
		if id != usage.Unattributed {
			rest = append(rest, id)
		}
	}
	sort.Strings(rest)
	out = append(out, rest...)
	if in[usage.Unattributed] {
		out = append(out, usage.Unattributed)
	}
	return out
}

// labelOf is the label of an account with no email in it.
func labelOf(order []Account, id string) string {
	label := id
	for _, a := range order {
		if a.ID == id {
			label = a.Label
		}
	}
	label = clean(label, maxName)
	if i := strings.Index(label, "@"); i >= 0 {
		if label = strings.TrimSpace(label[:i]); label == "" {
			label = "account"
		}
	}
	return label
}
