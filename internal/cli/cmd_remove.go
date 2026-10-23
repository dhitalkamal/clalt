package cli

import (
	"fmt"

	"github.com/dhitalkamal/clalt/internal/account"
)

const removeUsage = "clalt remove <name> [--force]"

// cmdRemove signs an account out and hides it. the folder and its history stay,
// and logging in under the same name brings the account back.
func (a *App) cmdRemove(args []string) error {
	label, force, err := parseRemoveArgs(args)
	if err != nil {
		return err
	}
	acc, err := a.findAccount(label)
	if err != nil {
		return err
	}
	if acc.Meta.Hidden {
		return fmt.Errorf("%q is already removed. 'clalt login %s' brings it back", label, label)
	}
	if !force {
		yes, err := a.confirm(fmt.Sprintf("remove %s? it is signed out and hidden, its history stays", shown(acc)))
		if err != nil {
			a.printf("\n")
			return fmt.Errorf("no answer was given, so %s was not removed. pass --force to remove without asking", shown(acc))
		}
		if !yes {
			a.printf("aborted\n")
			return nil
		}
	}
	if err := a.removeAccount(acc); err != nil {
		return err
	}
	if noFolderFor(acc) {
		a.printf("removed %s. it has no folder, since its id cannot be a folder name, so 'clalt login %s' cannot bring it back: sign in under another name\n",
			shown(acc), shown(acc))
		return nil
	}
	a.printf("removed %s. its history stays, and 'clalt login %s' brings it back\n", shown(acc), shown(acc))
	return nil
}

func parseRemoveArgs(args []string) (label string, force bool, err error) {
	var rest []string
	for _, arg := range args {
		if arg == "--force" || arg == "-f" {
			force = true
			continue
		}
		rest = append(rest, arg)
	}
	label, err = oneAccountArgs(rest, removeUsage)
	return label, force, err
}

// removeAccount signs the folder out unless claude says plainly that nobody is
// signed in, and only then hides the account, so no login is left behind in a
// folder nobody lists any more. any other answer may be a stored login that
// something else masks, such as an apiKeyHelper or a provider, and counts.
func (a *App) removeAccount(acc account.Account) error {
	if noFolderFor(acc) {
		return a.Store.Hide(acc.Name)
	}
	folder, err := folderFor(acc)
	if err != nil {
		return err
	}
	st, err := liveStatus(folder)
	if err != nil {
		return fmt.Errorf("could not check whether %s is signed in: %w. nothing was changed", shown(acc), err)
	}
	signedOut := false
	// the answer is in hand, so only what it says is left to ask
	if loginMayBeIn(st, nil) {
		if err := logoutFolder(folder); err != nil {
			return signOutFailed(acc, err, "the account was not removed", "clalt remove")
		}
		signedOut = true
	}
	if err := a.Store.Hide(acc.Name); err != nil {
		if signedOut {
			return fmt.Errorf("%s is signed out, but it is not removed: clalt could not save that: %w. 'clalt remove %s' tries again",
				shown(acc), err, shown(acc))
		}
		return err
	}
	return nil
}
