package cli

import (
	"errors"
	"fmt"

	"github.com/dhitalkamal/clalt/internal/account"
)

// advisedError is an error whose text already says how to sign the folder out.
type advisedError struct{ error }

func (e advisedError) Unwrap() error { return e.error }

// loginMayRemain asks claude after a sign-in that did not finish. claude can fail
// after it saved the login, so a failed sign-in is not proof that the folder has
// none: see loginMayBeIn.
func loginMayRemain(folder string) bool {
	st, err := liveStatus(folder)
	return loginMayBeIn(st, err)
}

// keepListed is for a failed sign-in that may have left a login in the folder of
// an account that was not listed: one this login made, or a removed one. a
// hidden account is one nobody can sign out, so a removed account is listed
// again, and the error says how to sign the folder out.
func (a *App) keepListed(acc account.Account, made bool, cause error) error {
	label := shown(acc)
	where := "stays in the list"
	if !made {
		if err := a.Store.Unhide(acc.Name); err != nil {
			return fmt.Errorf("%w. a login may be left in the folder and clalt could not list %s again (%v). 'clalt logout %s' signs it out",
				cause, label, err, label)
		}
		where = "is back in the list"
	}
	var advised advisedError
	if errors.As(cause, &advised) {
		return cause
	}
	return fmt.Errorf("%w. a login may be left in the folder, so %s %s. 'clalt logout %s' signs it out", cause, label, where, label)
}
