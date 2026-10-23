package cli

import (
	"errors"
	"fmt"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/claudecli"
)

// cmdLogout signs an account out of claude and keeps everything else: the
// folder, the history and the email, so the next sign-in is still checked
// against it. it works on a removed account too, whose folder can hold a login
// that a sign-in left behind, and leaves it removed.
func (a *App) cmdLogout(args []string) error {
	label, err := oneAccountArgs(args, "clalt logout <name>")
	if err != nil {
		return err
	}
	acc, err := a.findAccount(label)
	if err != nil {
		return err
	}
	if err := a.signOut(acc); err != nil {
		return err
	}
	if noFolderFor(acc) {
		a.printf("%s has no folder that clalt can use, so there is nothing to sign out\n", shown(acc))
		return nil
	}
	if acc.Meta.Hidden {
		a.printf("%s is signed out. it stays removed, and 'clalt login %s' brings it back\n", shown(acc), shown(acc))
		return nil
	}
	a.printf("%s is signed out. its history stays. sign in again with: clalt login %s\n", shown(acc), shown(acc))
	return nil
}

// signOut is the part of logout that dashboards share. if claude cannot sign
// the folder out, nothing is recorded, so the account does not look signed out
// while a login is still there.
func (a *App) signOut(acc account.Account) error {
	if noFolderFor(acc) {
		return a.Store.ClearSignIn(acc.Name)
	}
	folder, err := folderFor(acc)
	if err != nil {
		return err
	}
	if err := logoutFolder(folder); err != nil {
		return signOutFailed(acc, err, "clalt changed nothing it keeps", "clalt logout")
	}
	if err := a.Store.ClearSignIn(acc.Name); err != nil {
		return fmt.Errorf("%s is signed out, but clalt could not save that: %w. 'clalt logout %s' tries again",
			shown(acc), err, shown(acc))
	}
	return nil
}

// signOutFailed is the error for a sign-out that did not work. when claude
// could not say how it ended, the folder may be signed out already, so saying
// that nothing was changed would not be true of it.
func signOutFailed(acc account.Account, err error, kept, retry string) error {
	if errors.Is(err, claudecli.ErrSignOutUnknown) {
		return fmt.Errorf("could not sign %s out: %w. %s, but the folder may be signed out already: 'clalt list' shows whether it still is, '%s %s' tries again",
			shown(acc), err, kept, retry, shown(acc))
	}
	return fmt.Errorf("could not sign %s out: %w. nothing was changed", shown(acc), err)
}
