// Package defaulthome recognises the default claude home (~/.claude, what plain
// claude uses) as the login of one of clalt's accounts, by email, and keeps the
// answer in a small cache so that hooks and reports never have to ask claude.
package defaulthome

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/claudecli"
	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// State is how the default home relates to clalt's accounts.
type State string

// Matched: a visible account has the email of the default home. Outside: plain
// claude is signed in as somebody clalt does not know, or in a way that cannot
// be tied to an account. Unusable: claude could not be asked.
const (
	Matched  State = "matched"
	Outside  State = "outside"
	Unusable State = "unusable"
)

// Cache is default-home.json.
type Cache struct {
	State   State  `json:"state"`
	Account string `json:"account,omitempty"`
	Email   string `json:"email,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Off     bool   `json:"attributeOff,omitempty"`
	// MatchedSince is when this account became the match, as clalt saw it. it
	// resets whenever the state or the account changes, so a login that changed
	// is never charged for what came before.
	MatchedSince time.Time `json:"matchedSince,omitzero"`
	CheckedAt    time.Time `json:"checkedAt"`
}

// Active says whether plain claude counts as the account's: a match that names
// the account.
func (c Cache) Active() bool { return c.State == Matched && c.Account != "" }

// Since is when the current match started, falling back to the check time for a
// cache written before MatchedSince existed.
func (c Cache) Since() time.Time {
	if c.MatchedSince.IsZero() {
		return c.CheckedAt
	}
	return c.MatchedSince
}

// Candidate is a visible account as matching sees it.
type Candidate struct{ ID, Email string }

// Decide turns what claude said about home into a Cache. err is the failure of
// the call itself; an answer that says "signed out" is a normal one.
func Decide(st claudecli.AuthStatus, err error, home string, accts []Candidate, now time.Time) Cache {
	c := Cache{CheckedAt: now}
	if err != nil {
		c.State, c.Reason = Unusable, "claude could not be asked: "+err.Error()
		return c
	}
	if ok, _, text := st.SignedIn(home); !ok {
		c.State, c.Reason = Outside, "plain claude is "+text
		return c
	}
	c.Email = st.Email
	var hits []string
	for _, a := range accts {
		if a.Email != "" && strings.EqualFold(a.Email, st.Email) {
			hits = append(hits, a.ID)
		}
	}
	switch len(hits) {
	case 1:
		c.State, c.Account = Matched, hits[0]
	case 0:
		c.State, c.Reason = Outside, "no account has the email "+st.Email
	default:
		c.State, c.Reason = Outside, "more than one account has the email "+st.Email+": "+strings.Join(hits, ", ")
	}
	return c
}

// Merge is the cache after a refresh gave fresh. a failed check keeps an earlier
// match, since claude not answering is not a sign-out, and says why it failed.
// the user's choice to turn attribution off outlives every refresh.
func Merge(old, fresh Cache) Cache {
	if fresh.State == Unusable && old.State == Matched {
		old.Reason, old.CheckedAt = fresh.Reason, fresh.CheckedAt
		return old
	}
	fresh.Off = old.Off
	if fresh.State == Matched {
		fresh.MatchedSince = fresh.CheckedAt
		if old.State == Matched && old.Account == fresh.Account {
			fresh.MatchedSince = old.Since()
		}
	}
	return fresh
}

// Read reads the cache. a missing, corrupt or empty file is "not there".
func Read(path string) (Cache, bool) {
	raw, err := fsutil.ReadFile(path)
	if err != nil {
		return Cache{}, false
	}
	var c Cache
	if json.Unmarshal(raw, &c) != nil || c.State == "" {
		return Cache{}, false
	}
	return c, true
}

// Write saves the cache atomically.
func Write(path string, c Cache) error {
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, append(raw, '\n'), 0o600)
}
