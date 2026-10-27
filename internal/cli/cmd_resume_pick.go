package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/pick"
)

// pickForResume ranks the accounts for the folder a conversation ran in and
// returns the best one with the line that says why, which the caller prints. the sign-in is not asked of claude here: the run that
// follows makes every check it always makes.
func (a *App) pickForResume(cwd string) (account.Account, string, error) {
	accts, err := a.visibleAccounts()
	if err != nil {
		return account.Account{}, "", err
	}
	if len(accts) == 0 {
		return account.Account{}, "", errors.New("there are no accounts yet. sign one in with: clalt login <name>")
	}
	allRules, err := a.Store.AllRules()
	if err != nil {
		return account.Account{}, "", err
	}
	cands := make([]pick.Candidate, len(accts))
	for i, acc := range accts {
		r := allRules[acc.Name]
		cands[i] = pick.Candidate{
			ID: acc.Name, Label: acc.Label(), SignedIn: !acc.Meta.SignedInAt.IsZero() && acc.Meta.Email != "",
			Folders: r.Folders, AutoSwitch: r.AutoSwitch,
		}
	}
	res := pick.Rank(cands, pick.Options{Cwd: cwd, Now: a.clock(), Limits: feedLimits})
	if len(res.Ranked) == 0 {
		var why []string
		for _, l := range res.Left {
			why = append(why, clean(l.Label)+": "+clean(l.Why))
		}
		return account.Account{}, "", usageError{fmt.Sprintf("no account can be picked for this conversation (%s), pass --account <name>",
			strings.Join(why, "; ")), resumeUsage}
	}
	first := res.Ranked[0]
	note := fmt.Sprintf("picked %s: %s", clean(first.Label), first.Reason)
	for _, acc := range accts {
		if acc.Name == first.ID {
			return acc, note, nil
		}
	}
	return account.Account{}, "", errors.New("the picked account vanished")
}
