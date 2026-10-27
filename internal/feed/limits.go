package feed

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
)

func (s Store) limitsPath(account string) string {
	return filepath.Join(s.Dir, "accounts", account+".json")
}

// resetTolerance is how far two reset times may be apart and still be the same
// window: claude derives them from clocks of its own.
const resetTolerance = 60 * time.Second

// mergeWindow folds an observation into what is known. a window that has
// reset is dropped. a later reset time is a newer window and wins; inside one
// window the use only grows, so the higher value wins, which keeps a session
// that was idle and re-sends old numbers from lowering what another one saw.
func mergeWindow(old, obs *Window, now time.Time) *Window {
	if old != nil && old.Expired(now) {
		old = nil
	}
	if obs != nil && obs.Expired(now) {
		obs = nil
	}
	switch {
	case obs == nil:
		return old
	case old == nil, obs.ResetsAt.Sub(old.ResetsAt) > resetTolerance:
		return obs
	case obs.ResetsAt.Sub(old.ResetsAt) >= -resetTolerance && obs.UsedPct > old.UsedPct:
		return obs
	}
	return old
}

// MergeLimits records what a session saw of the account's two windows. nothing
// observed writes nothing.
func (s Store) MergeLimits(account string, five, seven *Window, now time.Time) error {
	if five == nil && seven == nil {
		return nil
	}
	if err := config.CheckAccountID(account); err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	cur, _ := s.Limits(account)
	cur.Account = account
	cur.FiveHour = mergeWindow(cur.FiveHour, five, now)
	cur.SevenDay = mergeWindow(cur.SevenDay, seven, now)
	cur.ObservedAt = now
	return writeJSON(s.limitsPath(account), cur)
}

// Limits reads the limits of an account. missing or corrupt is "not there".
func (s Store) Limits(account string) (AccountLimits, bool) {
	var l AccountLimits
	if config.CheckAccountID(account) != nil || !readJSON(s.limitsPath(account), &l) {
		return AccountLimits{}, false
	}
	return l, true
}

// AllLimits reads the limits of every account that has any, ordered by account.
func (s Store) AllLimits() ([]AccountLimits, error) {
	files, err := os.ReadDir(filepath.Join(s.Dir, "accounts"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []AccountLimits
	for _, f := range files {
		if id, ok := strings.CutSuffix(f.Name(), ".json"); ok {
			if l, ok := s.Limits(id); ok {
				out = append(out, l)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Account < out[j].Account })
	return out, nil
}
