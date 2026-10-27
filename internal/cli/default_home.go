package cli

import (
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/defaulthome"
	"github.com/dhitalkamal/clalt/internal/feedinstall"
	"github.com/dhitalkamal/clalt/internal/usage"
)

// refreshDefaultHome asks claude who plain claude is signed in as, matches that
// against the visible accounts and caches the answer. claude gets no short
// deadline, see claudecli.Status. only doctor, feed and the dashboard start call
// this: hooks and reports read the cache.
func (a *App) refreshDefaultHome() (defaulthome.Cache, error) { return a.refreshDefault(true) }

// refreshDefault is refreshDefaultHome. flush says whether a match that is
// about to change first gets the sessions it has earned written to the ledger,
// which scans the transcripts: a hook has no time for that, and what it skips
// stays unattributed.
func (a *App) refreshDefault(flush bool) (defaulthome.Cache, error) {
	home := config.SharedClaudeDir()
	if home == "" {
		return defaulthome.Cache{State: defaulthome.Unusable, Reason: noHomeKnown, CheckedAt: a.clock()}, nil
	}
	accts, err := a.visibleAccounts()
	if err != nil {
		return defaulthome.Cache{}, err
	}
	cands := make([]defaulthome.Candidate, len(accts))
	for i, acc := range accts {
		cands[i] = defaulthome.Candidate{ID: acc.Name, Email: acc.Meta.Email}
	}
	st, serr := sharedStatus(home)
	fresh := defaulthome.Decide(st, serr, home, cands, a.clock())
	old, _ := defaulthome.Read(config.DefaultHomePath())
	c := defaulthome.Merge(old, fresh)
	if flush && old.Active() && (c.State != old.State || c.Account != old.Account) {
		a.flushDefaultLines(old, old.CheckedAt)
	}
	if err := os.MkdirAll(filepath.Dir(config.DefaultHomePath()), 0o700); err != nil {
		return c, err
	}
	return c, defaulthome.Write(config.DefaultHomePath(), c)
}

// defaultHomeAccount is the id of the account that plain claude is signed in as,
// from the cache, or "". it never asks claude. a match whose account is gone or
// removed since counts for nothing.
func (a *App) defaultHomeAccount() string {
	c, ok := defaulthome.Read(config.DefaultHomePath())
	if !ok || !c.Active() {
		return ""
	}
	accts, err := a.visibleAccounts()
	if err != nil {
		return ""
	}
	for _, acc := range accts {
		if acc.Name == c.Account {
			return c.Account
		}
	}
	return ""
}

// addDefaultLines gives the sessions clalt never saw to the account of the
// default home, and writes that into the ledger as such. attribution the user
// turned off, or a default home that matches no account, adds nothing. only
// sessions that started since the match began are given: a login that changed
// is not charged for what ran under the one before.
func (a *App) addDefaultLines(records []usage.Record, sessions []usage.SessionAccount, cutover time.Time) ([]usage.SessionAccount, error) {
	c, _ := defaulthome.Read(config.DefaultHomePath())
	account := a.defaultHomeAccount()
	if account == "" || c.Off {
		return sessions, nil
	}
	lines := usage.DefaultLines(records, sessions, laterOf(cutover, c.Since()), account)
	for _, l := range lines {
		if err := usage.AppendSessionAccount(config.SessionAccountsPath(), l); err != nil {
			return sessions, err
		}
	}
	return append(sessions, lines...), nil
}

func laterOf(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// flushDefaultLines writes the sessions that the match in old earned, before a
// refresh replaces it, which is the sessions that started before until, the time
// of the change. best effort: a failure leaves them unattributed.
func (a *App) flushDefaultLines(old defaulthome.Cache, until time.Time) {
	if old.Off {
		return
	}
	projects := config.ClaudeProjectsDir()
	cutover, err := a.Store.ThinSince()
	if projects == "" || err != nil {
		return
	}
	records, err := usage.ScanRoots([]string{projects})
	if err != nil {
		return
	}
	sessions, err := usage.ReadSessionAccounts(config.SessionAccountsPath())
	if err != nil {
		return
	}
	records = slices.DeleteFunc(records, func(r usage.Record) bool { return !r.Timestamp.Before(until) })
	for _, l := range usage.DefaultLines(records, sessions, laterOf(cutover, old.Since()), old.Account) {
		if usage.AppendSessionAccount(config.SessionAccountsPath(), l) != nil {
			return
		}
	}
}

// defaultHomeChecks are the doctor lines about plain claude: which account, if
// any, it is signed in as, and whether the feed is installed for it. doctor only
// looks, so this reads the cache that feed status, feed install, usage and the
// dashboard refresh, and asks claude nothing.
func (a *App) defaultHomeChecks() []checkResult {
	const scope = "default home"
	c, ok := defaulthome.Read(config.DefaultHomePath())
	if !ok {
		return []checkResult{result(scope, "match", statusOK, "plain claude (~/.claude) has not been checked yet, clalt feed status does it")}
	}
	age := "checked " + ago(c.CheckedAt, a.clock())
	switch {
	case c.Active():
		label := a.accountLabel(c.Account)
		out := []checkResult{result(scope, "match", statusOK, "plain claude (~/.claude) is signed in as %s, so it is the account %s (%s)", c.Email, label, age)}
		opts, oerr := a.feedOptions()
		if oerr != nil {
			return out
		}
		st, cerr := feedinstall.Check(opts)
		switch {
		case cerr != nil:
			return append(out, result(scope, "feed", statusWarn, "could not check %s: %v", opts.Settings, cerr))
		case st.Installed && st.BinMissing:
			return append(out, result(scope, "feed", statusWarn, "the clalt that the feed commands run, %s, no longer exists: clalt feed uninstall, then clalt feed install", st.Bin))
		case st.Installed:
			return append(out, result(scope, "feed", statusOK, "the live feed is installed in %s", opts.Settings))
		}
		return append(out, result(scope, "feed", statusOK, "the live feed is not installed for plain claude, clalt feed install adds it"))
	case c.State == defaulthome.Unusable:
		return []checkResult{result(scope, "match", statusWarn, "plain claude (~/.claude) could not be checked: %s (%s)", c.Reason, age)}
	}
	return []checkResult{result(scope, "match", statusOK, "plain claude (~/.claude) stays outside clalt: %s (%s)", c.Reason, age)}
}

// staleDefaultHome is how old the cache may get before usage and the dashboard
// ask claude again.
const staleDefaultHome = 24 * time.Hour

// refreshDefaultHomeIfStale refreshes the cache when there is none or it is old.
// a failure is not the caller's: the cache keeps what it had.
func (a *App) refreshDefaultHomeIfStale() {
	if c, ok := defaulthome.Read(config.DefaultHomePath()); ok && a.clock().Sub(c.CheckedAt) < staleDefaultHome {
		return
	}
	_, _ = a.refreshDefaultHome()
}
