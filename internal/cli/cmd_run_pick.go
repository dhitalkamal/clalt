package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/pick"
)

// feedLimits looks up the limits the feed last saw for an account. the feed is
// step 3a's and is not wired in yet, so until it is nothing is known and the
// accounts rank by label. a variable so that a test can make it answer.
var feedLimits = func(accountID string) (pick.Limits, bool) { return pick.Limits{}, false }

// pickAccount chooses the account that `clalt run` uses when it is given none,
// and says on stderr which it picked and why. it is the allowed account with the
// most room, see package pick. the sign-in recorded in the state only decides
// who is looked at: the one that wins is asked of claude, since a login can be
// gone since it was recorded, and the next one is tried when it is.
func (a *App) pickAccount() (account.Account, error) {
	accts, err := a.visibleAccounts()
	if err != nil {
		return account.Account{}, err
	}
	if len(accts) == 0 {
		return account.Account{}, errors.New("there are no accounts yet. sign one in with: clalt login <name>")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return account.Account{}, fmt.Errorf("could not tell which folder this is: %w", err)
	}
	allRules, err := a.Store.AllRules()
	if err != nil {
		return account.Account{}, err
	}
	byID := make(map[string]account.Account, len(accts))
	cands := make([]pick.Candidate, len(accts))
	for i, acc := range accts {
		byID[acc.Name] = acc
		r := allRules[acc.Name]
		cands[i] = pick.Candidate{
			ID: acc.Name, Label: acc.Label(), SignedIn: !acc.Meta.SignedInAt.IsZero() && acc.Meta.Email != "",
			Folders: r.Folders, AutoSwitch: r.AutoSwitch,
		}
	}

	var out []string
	for {
		res := pick.Rank(cands, pick.Options{Cwd: cwd, Now: a.clock(), Limits: feedLimits})
		if len(out) == 0 {
			for _, l := range res.Left {
				out = append(out, clean(l.Label)+": "+clean(l.Why))
			}
		}
		if len(res.Ranked) == 0 {
			return account.Account{}, fmt.Errorf("no account can be picked for %s: %s. run one by name with: clalt run <name>",
				clean(cwd), strings.Join(out, "; "))
		}
		first := res.Ranked[0]
		if why := a.notReady(byID[first.ID]); why != "" {
			fmt.Fprintf(a.Err, "skipped %s: %s\n", clean(first.Label), why)
			out = append(out, clean(first.Label)+": "+why)
			cands = withoutCandidate(cands, first.ID)
			continue
		}
		fmt.Fprintf(a.Err, "picked %s: %s\n", clean(first.Label), first.Reason)
		return byID[first.ID], nil
	}
}

// notReady asks claude whether the account is signed in as the person it was
// set up for, and says what is wrong when it is not, with how to fix it.
func (a *App) notReady(acc account.Account) string {
	folder, err := folderFor(acc)
	if err != nil {
		return clean(err.Error())
	}
	st, err := liveStatus(folder)
	if err != nil {
		return "could not check the sign-in: " + clean(err.Error())
	}
	if ok, why, text := checkSignIn(st, folder, acc.Meta); !ok {
		return text + ". " + loginAdvice(why, shown(acc))
	}
	return ""
}

func withoutCandidate(cands []pick.Candidate, id string) []pick.Candidate {
	out := make([]pick.Candidate, 0, len(cands))
	for _, c := range cands {
		if c.ID != id {
			out = append(out, c)
		}
	}
	return out
}
