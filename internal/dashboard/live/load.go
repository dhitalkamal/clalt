package live

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/dashboard"
)

// loginLife is how long a sign-in lasts: claude caps its refresh grants there
const loginLife = 30 * 24 * time.Hour

// signResult is what the sign-in cache of an account has to say
type signResult struct {
	in   SignIn
	have bool
	err  error
}

// Load reads everything the dashboard shows. only the state file is needed:
// whatever else cannot be read is a warning, and what a background read has
// not brought yet is shown from the record.
func (a *Adapter) Load(ctx context.Context) (dashboard.Snapshot, error) {
	all, err := a.cfg.Store.List()
	if err != nil {
		return dashboard.Snapshot{}, err
	}
	accts := visible(all)
	now := a.cfg.Now()
	var warns []string

	// everything slow is started first and waited for against one deadline, so
	// a cold load waits one grace and not three
	deadline := time.Now().Add(a.cfg.Grace)
	a.ledger.get(ctx, 0)
	ver, haveVer, _ := a.version.get(ctx, a.cfg.Grace)
	usable := !haveVer || ver.err == nil
	if haveVer && ver.err != nil {
		warns = append(warns, "claude cannot be used: "+ver.err.Error()+". sign-in states are from the record")
	}
	signs := a.signIns(ctx, accts, usable, deadline)

	led, haveLed, ledErr := a.ledger.get(ctx, time.Until(deadline))
	switch {
	case ledErr != nil:
		warns = append(warns, "usage could not be read: "+ledErr.Error())
	case !haveLed:
		warns = append(warns, "usage is still being read")
	case led.unattributed >= 0.005:
		warns = append(warns, fmt.Sprintf("~$%.2f of the last %d days is not split per account yet. clalt usage lists it as unattributed", led.unattributed, historyDays))
	}
	if led == nil {
		led = buildLedger(nil, now, a.cfg.Home)
	}

	byAccount, w := a.sessionsByAccount(ctx, now, led)
	warns = append(warns, w...)
	if _, err := a.cfg.Gateways.Running(ctx); err != nil {
		warns = append(warns, "crew gateways could not be read: "+err.Error())
	}

	snap := dashboard.Snapshot{Warnings: warns}
	for _, acc := range accts {
		built, warn := a.buildAccount(ctx, acc, signs[acc.Name], ver, led, byAccount[acc.Name], now)
		snap.Accounts = append(snap.Accounts, built)
		if warn != "" {
			snap.Warnings = append(snap.Warnings, warn)
		}
	}
	return snap, nil
}

// visible are the accounts that were not removed, by label ignoring case and
// then by id, like `clalt list`
func visible(all []account.Account) []account.Account {
	var out []account.Account
	for _, acc := range all {
		if !acc.Meta.Hidden {
			out = append(out, acc)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		li, lj := strings.ToLower(out[i].Label()), strings.ToLower(out[j].Label())
		if li != lj {
			return li < lj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// signIns reads the sign-in caches. every check is started first, so the waits
// for the ones that have no answer yet overlap and share one grace.
func (a *Adapter) signIns(ctx context.Context, accts []account.Account, usable bool, deadline time.Time) map[string]signResult {
	out := make(map[string]signResult, len(accts))
	if !usable {
		return out
	}
	var todo []account.Account
	for _, acc := range accts {
		if config.CheckAccountID(acc.Name) != nil {
			continue
		}
		a.entry(acc).c.get(ctx, 0)
		todo = append(todo, acc)
	}
	for _, acc := range todo {
		in, have, err := a.entry(acc).c.get(ctx, time.Until(deadline))
		out[acc.Name] = signResult{in: in, have: have, err: err}
	}
	return out
}

func (a *Adapter) buildAccount(ctx context.Context, acc account.Account, sr signResult, ver versionInfo, led *ledger, sessions []dashboard.Session, now time.Time) (dashboard.Account, string) {
	m := acc.Meta
	out := dashboard.Account{
		ID: acc.Name, Label: acc.Label(), Email: m.Email, Org: m.Org, Plan: planText(m.Plan),
		LastUsedAt: m.LastUsedAt, LastIn: led.lastIn[acc.Name],
		Budget:   dashboard.Budget{Daily: m.DailyBudget, Weekly: m.WeeklyBudget},
		Sessions: sessions, History: led.histories[acc.Name],
	}
	folder, folderErr := config.AccountDirFor(acc.Name)
	out.Profile = dashboard.Profile{Folder: folder, ClaudeVersion: ver.version, FeedInstalled: a.cfg.Feed.Installed(acc.Name)}
	var warn string
	if folderErr == nil {
		var err error
		if out.Profile.SharedSettings, err = a.cfg.Host.LinksInSync(folder); err != nil {
			out.Profile.SharedSettings = false
			warn = "could not check the links of " + acc.Label() + ": " + err.Error()
		}
	}
	out.SignedIn, out.Problem = signState(acc, sr, folderErr)
	if sr.have && sr.in.OK {
		out.Email, out.Org, out.Plan = orElse(sr.in.Email, m.Email), orElse(sr.in.Org, m.Org), orElse(planText(sr.in.Plan), out.Plan)
	}
	if !out.SignedIn {
		out.Plan, out.Org = "", ""
		return out, warn
	}
	if !m.SignedInAt.IsZero() {
		out.Login = dashboard.Login{SignedInAt: m.SignedInAt, DueAt: m.SignedInAt.Add(loginLife)}
	}
	lim, ok, err := a.cfg.Feed.Limits(ctx, acc.Name)
	var readings []Reading
	if ok && err == nil {
		readings, err = a.cfg.Feed.Readings(ctx, acc.Name, now.AddDate(0, 0, -chartDays))
	}
	if err != nil {
		ok = false
	}
	out.Limits = mapLimits(lim, ok, readings, now)
	return out, warn
}

// signState says whether the account can be used, and what is wrong when
// something is. a check that has not answered, or failed, leaves the record.
func signState(acc account.Account, sr signResult, folderErr error) (signedIn bool, problem string) {
	recorded := !acc.Meta.SignedInAt.IsZero()
	switch {
	case folderErr != nil:
		return false, folderErr.Error()
	case !sr.have && sr.err != nil:
		return recorded, "could not check the sign-in: " + sr.err.Error()
	case !sr.have:
		return recorded, ""
	}
	if sr.err != nil {
		problem = "could not check the sign-in again: " + sr.err.Error()
	}
	switch {
	case sr.in.OK:
		return true, problem
	case problem == "" && sr.in.Reason != notSignedIn:
		problem = sr.in.Text
	}
	return false, problem
}

func planText(plan string) string {
	if plan == "" {
		return ""
	}
	return plan + " plan"
}

func orElse(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}
