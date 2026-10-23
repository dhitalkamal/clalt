package cli

// removedChecks look at the folders of removed accounts. a removed account has
// been signed out, but a login can be left in its folder, after a sign-in that
// failed or one made behind clalt's back, and nobody lists the account any more.
// there is a line for a folder that still holds a claude.ai login and for one
// claude could not be asked about, and none when all is well. claude is only
// asked about a folder that is there.
func (a *App) removedChecks(claudeUsable bool) []checkResult {
	if !claudeUsable {
		return nil
	}
	removed, err := a.removedAccounts()
	if err != nil {
		// the list could not be read, which the checks of the listed accounts say
		return nil
	}
	var gone []accountFolder
	for _, acc := range removed {
		// the folder of a visible account whose id differs only in case is the
		// same folder here, and the login in it is that account's: logging the
		// removed one out would sign it out
		if others, err := a.foldersSharedWith(acc); err == nil && len(others) > 0 {
			continue
		}
		if folder, err := folderFor(acc); err == nil {
			gone = append(gone, accountFolder{acc.Label(), folder})
		}
	}
	answers := make([]answer, len(gone))
	parallelDo(len(gone), statusWorkers, func(i int) {
		answers[i].status, answers[i].err = liveStatus(gone[i].folder)
	})
	var out []checkResult
	for i, g := range gone {
		switch {
		case answers[i].err != nil:
			out = append(out, result(g.label, "removedLogin", statusWarn,
				"could not check whether the removed account is still signed in: %v", answers[i].err))
		// a login that something else supplies, an apiKeyHelper in the shared
		// settings for one, is reported for every folder there is, and a
		// sign-out does not remove it: only the login that logout can remove is
		// worth a line here, unlike when remove decides whether to sign out
		case holdsClaudeAiLogin(answers[i].status):
			out = append(out, result(g.label, "removedLogin", statusWarn,
				"removed but its folder is still signed in: clalt logout %s", g.label))
		}
	}
	return out
}

// accountFolder is an account's label and where its folder is.
type accountFolder struct{ label, folder string }
