package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/claudecli"
	"github.com/dhitalkamal/clalt/internal/config"
)

// loginLife is how long a sign-in lasts: claude caps its refresh grants there.
const loginLife = 30 * 24 * time.Hour

func loginDue(signedIn time.Time) time.Time { return signedIn.Add(loginLife) }

func dateOf(t time.Time) string { return t.Local().Format("2006-01-02") }

// sameEmail compares addresses ignoring case. an empty one is unknown, and
// unknown is not equal to anything, not even to another unknown.
func sameEmail(a, b string) bool { return a != "" && b != "" && strings.EqualFold(a, b) }

// parenthesized joins what is known into "(max, Example Org)", or nothing.
func parenthesized(parts ...string) string {
	var known []string
	for _, p := range parts {
		if p != "" {
			known = append(known, p)
		}
	}
	if len(known) == 0 {
		return ""
	}
	return " (" + strings.Join(known, ", ") + ")"
}

// checkSignIn is the one rule for whether an account can be used: claude must
// report a claude.ai login in the account's own folder, and it must be the
// email that was recorded when the account signed in. why is claude's reason,
// and ReasonNone when the login is fine and it is the recorded identity that
// does not fit.
func checkSignIn(st claudecli.AuthStatus, folder string, meta account.Meta) (ok bool, why claudecli.Reason, text string) {
	if ok, why, text := signedInHere(st, folder); !ok {
		return false, why, text
	}
	switch {
	case meta.Email == "":
		return false, claudecli.ReasonNone, "no sign-in is recorded for this account"
	case !sameEmail(st.Email, meta.Email):
		return false, claudecli.ReasonNone, "signed in as a different account than the one recorded"
	}
	return true, claudecli.ReasonNone, ""
}

// loginAdvice is what to do about a folder that does not count as signed in.
// signing in again changes nothing when something outside the account, such as
// an apiKeyHelper in the shared settings, overrides its login.
func loginAdvice(why claudecli.Reason, label string) string {
	switch why {
	case claudecli.ReasonOtherAuthMethod, claudecli.ReasonOtherProvider:
		return "something outside the account overrides its login. 'clalt doctor' checks the shell and the shared settings for it"
	}
	return "sign in with: clalt login " + label
}

// noteStripped says which variables will not reach claude. names only. it is
// for whoever is at the terminal, so it goes to stderr and leaves stdout to
// claude.
func (a *App) noteStripped(folder string) {
	if _, removed := claudecli.Env(os.Environ(), folder); len(removed) > 0 {
		fmt.Fprintf(a.Err, "note: not passing these variables to claude, they would override the account's login: %s\n",
			strings.Join(removed, ", "))
	}
}

// folderFor is where an account's claude config lives. an id that came from an
// old state file was never checked, and it is joined into a path here, so it
// goes through the one check for that. the check is looser than the one for a
// new label: an old id that is still a safe folder name keeps working. for one
// that is not, remove is the way out, and the error says so.
func folderFor(acc account.Account) (string, error) {
	if err := config.CheckAccountID(acc.Name); err != nil {
		return "", fmt.Errorf("%w. clalt will not use it as a folder name. 'clalt remove %s' removes the account, and 'clalt login <name>' under another name makes a new one",
			err, acc.Label())
	}
	return config.AccountDirFor(acc.Name)
}

// shown is the label of an account as it is printed. a state file that an older
// clalt wrote was never checked, ids from before names were validated can be
// anything, and a label defaults to the id, so what is printed goes through clean.
func shown(acc account.Account) string { return clean(acc.Label()) }

// noFolderFor is true for an id that cannot be a folder name. no folder was ever
// made for one, so a command that only looks at the folder has nothing to do.
func noFolderFor(acc account.Account) bool { return config.CheckAccountID(acc.Name) != nil }

// folderExists says whether the folder for an id is already there. the store
// gives up on any error but "not there", so a folder that cannot be looked at is
// never taken for a free one.
func folderExists(id string) (bool, error) {
	dir, err := config.AccountDirFor(id)
	if err != nil {
		return false, err
	}
	return folderThere(dir)
}

// folderThere says whether a folder is there. only "not there" means absent: one
// that cannot be looked at is unknown, which is not the same as one without a
// login, and the caller has to say so instead of going ahead on a guess.
func folderThere(path string) (bool, error) {
	_, err := os.Lstat(path)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	}
	return false, err
}
