package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/registry"
)

// deleteAccount is remove --delete: check the account can go, ask unless
// forced, then carry the delete out.
func (a *App) deleteAccount(acc account.Account, force bool) error {
	folder, err := a.checkDelete(acc)
	if err != nil {
		return err
	}
	if !force {
		q := fmt.Sprintf("delete %s for good? it has no folder, only its record goes", shown(acc))
		if folder != "" {
			email := acc.Meta.Email
			if email == "" {
				email = "no email recorded"
			}
			q = fmt.Sprintf("delete %s (%s) for good? it is signed out, its record goes, and this folder is deleted: %s",
				shown(acc), email, folder)
		}
		if ok, err := a.askDelete(acc, q); !ok {
			return err
		}
	}
	if err := a.performDelete(acc, folder); err != nil {
		return err
	}
	if folder == "" {
		a.printf("deleted %s\n", shown(acc))
	} else {
		a.printf("deleted %s and its folder %s\n", shown(acc), folder)
	}
	return nil
}

// checkDelete is every refusal of a delete, before anything is asked or
// changed. folder is empty for an id that never had one. the dashboard calls
// it too, so both follow the same rules.
func (a *App) checkDelete(acc account.Account) (folder string, err error) {
	if err := a.refuseSharedFolder(acc); err != nil {
		return "", err
	}
	if err := a.refuseHiddenSharer(acc); err != nil {
		return "", err
	}
	if err := refuseLiveSession(acc); err != nil {
		return "", err
	}
	if noFolderFor(acc) {
		return "", nil
	}
	folder, err = folderFor(acc)
	if err != nil {
		return "", err
	}
	if err := checkDeletable(folder); err != nil {
		return "", fmt.Errorf("will not delete %s: %w. nothing was changed", shown(acc), err)
	}
	return folder, nil
}

// performDelete signs the account out, deletes its folder and drops its
// record, printing nothing. nothing is deleted unless the sign-out is known
// to have worked, and checkDelete must have passed first.
func (a *App) performDelete(acc account.Account, folder string) error {
	if folder == "" {
		return a.deleteNoFolder(acc)
	}
	if err := a.signOutForDelete(acc, folder); err != nil {
		return err
	}
	// RemoveAll deletes symlinks themselves and never follows them
	if err := os.RemoveAll(folder); err != nil {
		return fmt.Errorf("%s is signed out, but its folder %s could not be deleted: %w. the record is kept", shown(acc), folder, err)
	}
	if err := a.Store.Delete(acc.Name); err != nil {
		return fmt.Errorf("the folder of %s is deleted, but clalt could not drop its record: %w. 'clalt remove %s --delete' tries again",
			shown(acc), err, shown(acc))
	}
	return nil
}

// deleteNoFolder drops the record of an id that never had a folder.
func (a *App) deleteNoFolder(acc account.Account) error {
	return a.Store.Delete(acc.Name)
}

// askDelete asks the question. ok false with a nil error means the answer was no.
func (a *App) askDelete(acc account.Account, q string) (bool, error) {
	yes, err := a.confirm(q)
	if err != nil {
		a.printf("\n")
		return false, fmt.Errorf("no answer was given, so %s was not deleted. pass --force to delete without asking", shown(acc))
	}
	if !yes {
		a.printf("aborted\n")
	}
	return yes, nil
}

// signOutForDelete signs the folder out and stops on any doubt: if claude
// cannot say whether anyone is signed in, or the sign-out fails, nothing is
// deleted.
func (a *App) signOutForDelete(acc account.Account, folder string) error {
	st, err := liveStatus(folder)
	if err != nil {
		return fmt.Errorf("could not check whether %s is signed in: %w. nothing was deleted", shown(acc), err)
	}
	if loginMayBeIn(st, nil) {
		if err := logoutFolder(folder); err != nil {
			return signOutFailed(acc, err, "nothing was deleted", "clalt remove --delete")
		}
	}
	return nil
}

// checkDeletable says whether folder is a real directory (not a link) that sits
// directly under the accounts dir. a folder that is already gone passes.
func checkDeletable(folder string) error {
	// the comparison is lexical: it does not guard against ~/.config/clalt or
	// the accounts folder itself being a symlink
	if filepath.Dir(folder) != filepath.Clean(config.AccountsDir()) {
		return fmt.Errorf("%s is not inside clalt's accounts folder", folder)
	}
	fi, err := os.Lstat(folder)
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return err
	case !fi.IsDir():
		return fmt.Errorf("%s is a link or a file, not a folder", folder)
	}
	return nil
}

// refuseHiddenSharer looks at every record, hidden ones too: foldersSharedWith
// skips them, but a removed account that has the same folder would lose it.
func (a *App) refuseHiddenSharer(acc account.Account) error {
	groups, err := a.Store.CaseCollisions()
	if err != nil {
		return err
	}
	for _, g := range groups {
		if !slices.Contains(g, acc.Name) {
			continue
		}
		for _, id := range g {
			if id != acc.Name && foldersCollide(acc.Name, id) {
				return fmt.Errorf("%q has the same folder as %q, which would be deleted with it. nothing was changed",
					shown(acc), clean(id))
			}
		}
	}
	return nil
}

// refuseLiveSession stops a delete under a running session, even with --force:
// the session works in this very folder.
func refuseLiveSession(acc account.Account) error {
	recs, err := registry.Registry{Dir: config.SessionsDir()}.List()
	if err != nil {
		return fmt.Errorf("could not check for running sessions: %w. nothing was changed", err)
	}
	for _, r := range recs {
		if r.Account == acc.Name {
			pid := r.WrapperPID
			if pid == 0 {
				pid = r.PID
			}
			return fmt.Errorf("%s has a running session (pid %d). end the session first, then delete it. nothing was changed",
				shown(acc), pid)
		}
	}
	return nil
}
