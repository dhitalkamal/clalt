package cli

import (
	"fmt"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/claudecli"
)

const loginUsage = "clalt login <name>"

// cmdLogin signs an account in through claude's own login, inside the account's
// folder, and records who signed in. a name that is not an account yet makes
// one; a removed one comes back once the sign-in works.
func (a *App) cmdLogin(args []string) error {
	label, err := oneAccountArgs(args, loginUsage)
	if err != nil {
		return err
	}
	if err := refuseReservedLabel(label); err != nil {
		return err
	}
	acc, found, err := a.Store.ResolveLabel(label)
	if err != nil {
		return err
	}
	made := !found
	if made {
		if acc, err = a.createAccount(label); err != nil {
			return err
		}
	}
	loginRemains, err := a.signIn(acc)
	switch {
	case err == nil:
	case made && !loginRemains:
		a.hideUnused(acc)
	case loginRemains && (made || acc.Meta.Hidden):
		err = a.keepListed(acc, made, err)
	}
	return err
}

// hideUnused takes an account that this login made, and that nobody signed in
// to, out of the list. it is hidden and not deleted, so its folder is not
// orphaned and the next login under the same name reuses it.
func (a *App) hideUnused(acc account.Account) {
	if err := a.Store.Hide(acc.Name); err != nil {
		fmt.Fprintf(a.Err, "warning: could not take %s out of the list: %v\n", shown(acc), err)
	}
}

// signIn runs the sign-in and records who signed in. loginRemains is true when
// it failed at a point where claude may have left a login in the folder, or
// could not be asked: such an account has to stay in the list, since a hidden
// one cannot be signed out.
func (a *App) signIn(acc account.Account) (loginRemains bool, err error) {
	folder, err := folderFor(acc)
	if err != nil {
		return false, err
	}
	if err := a.refuseSharedFolder(acc); err != nil {
		return false, err
	}
	if _, err := a.syncAccount(folder); err != nil {
		return false, err
	}
	a.warnHistory(shown(acc), folder)

	a.noteStripped(folder)
	// advice for whoever is at the terminal, so not on stdout, which is for output
	fmt.Fprintf(a.Err, "tip: if your browser is signed into another claude.ai account, use a private window for this sign-in\n")
	login, err := startLogin(folder, acc.Meta.Email)
	if err != nil {
		return false, err
	}
	login.Stdin, login.Stdout, login.Stderr = a.In, a.Out, a.Err
	if err := login.Run(); err != nil {
		return loginMayRemain(folder), fmt.Errorf("claude auth login did not finish: %w", err)
	}

	// claude finished, so from here on there may be a login in the folder
	st, err := liveStatus(folder)
	if err != nil {
		return loginMayBeIn(st, err), err
	}
	if ok, _, text := signedInHere(st, folder); !ok {
		return loginMayBeIn(st, nil), fmt.Errorf("the sign-in did not complete: %s", text)
	}
	problem, err := a.identityProblem(acc, st)
	if err != nil {
		return true, err
	}
	if problem != "" {
		return a.refuseLogin(acc, folder, problem)
	}

	signedInAt := a.clock()
	err = a.Store.SetIdentity(acc.Name, account.Identity{
		Email: st.Email, Org: st.OrgName, Plan: st.SubscriptionType, SignedInAt: signedInAt,
	})
	if err != nil {
		return true, fmt.Errorf("the sign-in worked, but clalt could not record it: %w. 'clalt login %s' again records it", err, shown(acc))
	}
	if acc.Meta.Hidden {
		if err := a.Store.Unhide(acc.Name); err != nil {
			return true, fmt.Errorf("the sign-in worked, but clalt could not list %s again: %w. 'clalt login %s' tries again", shown(acc), err, shown(acc))
		}
	}
	a.printf("%s is signed in as %s%s\n", shown(acc), st.Email, parenthesized(st.SubscriptionType, st.OrgName))
	a.printf("sign in again by %s\n", dateOf(loginDue(signedInAt)))
	return false, nil
}

// identityProblem says why a sign-in as the email claude reports must not be
// kept for acc, or returns "" when it can be: it is a different person than the
// one recorded for the account, or the email already belongs to another visible
// account.
func (a *App) identityProblem(acc account.Account, st claudecli.AuthStatus) (string, error) {
	if acc.Meta.Email != "" && !sameEmail(acc.Meta.Email, st.Email) {
		return fmt.Sprintf("%q is set up for %s, but the sign-in was %s", shown(acc), acc.Meta.Email, st.Email), nil
	}
	list, err := a.Store.List()
	if err != nil {
		return "", err
	}
	for _, other := range list {
		if other.Name != acc.Name && !other.Meta.Hidden && sameEmail(other.Meta.Email, st.Email) {
			return fmt.Sprintf("%s already belongs to %q", st.Email, shown(other)), nil
		}
	}
	return "", nil
}

// refuseLogin signs the folder out again, so a login for the wrong person is
// not left behind, and tells the user why. loginRemains is true when the
// sign-out did not work.
func (a *App) refuseLogin(acc account.Account, folder, problem string) (loginRemains bool, err error) {
	if err := logoutFolder(folder); err != nil {
		return true, advisedError{fmt.Errorf("%s, and claude could not sign the folder out again (%v). run 'clalt logout %s'", problem, err, shown(acc))}
	}
	if err := a.Store.ClearSignIn(acc.Name); err != nil {
		return false, fmt.Errorf("%s. the folder was signed out again, but clalt could not save that: %w", problem, err)
	}
	return false, fmt.Errorf("%s. the folder was signed out again. use a private browser window and try again", problem)
}
