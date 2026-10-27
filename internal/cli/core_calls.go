package cli

import (
	"context"
	"os/exec"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/claudecli"
	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/profile"
)

// the calls that several commands make into claudecli (who is signed in, sign
// out, sign in, the claude version), into profile (the links and the mcp
// servers) and into the account store (make an account, change a label). each
// one carries what every caller needs: a folder that is not there is answered
// without asking claude, the timeout is the same, and what claude says is
// cleaned. a call that only one command makes is made in that command.

// authTimeout bounds one call to claude that is not interactive: auth status,
// auth logout, --version. it is a last resort for a claude that never answers,
// so it is long: real claude has no SIGINT handler, and cancelling ends it at
// once, possibly in the middle of a token refresh, which can sign the folder
// out. claudecli has the same two minutes as its default, this one is a
// variable so that a test can make it short.
var authTimeout = 2 * time.Minute

// liveStatus asks claude who is signed in to an account folder. claude would
// create a folder it is asked about, and claudecli refuses to ask, so a folder
// that is not there is answered here: it holds no login.
func liveStatus(folder string) (claudecli.AuthStatus, error) {
	there, err := folderThere(folder)
	switch {
	case err != nil:
		return claudecli.AuthStatus{}, err
	case !there:
		return claudecli.AuthStatus{AuthMethod: "none", ConfigDirectory: folder}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), authTimeout)
	defer cancel()
	st, err := claudecli.Status(ctx, folder)
	return cleanStatus(st), err
}

// sharedStatus asks claude who is signed in to the shared claude home, a read
// that claudecli allows only through its own call.
func sharedStatus(folder string) (claudecli.AuthStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), authTimeout)
	defer cancel()
	st, err := claudecli.StatusOfShared(ctx, folder)
	return cleanStatus(st), err
}

// cleanStatus makes the fields that clalt stores and shows safe. the fields it
// only compares, the auth method and the folder, are left exactly as claude
// said them, so that cleaning cannot turn a wrong answer into a right one.
func cleanStatus(st claudecli.AuthStatus) claudecli.AuthStatus {
	st.Email = clean(st.Email)
	st.OrgName = clean(st.OrgName)
	st.SubscriptionType = clean(st.SubscriptionType)
	st.Warning = clean(st.Warning)
	return st
}

// loginMayBeIn is the one rule for whether a login may be in a folder, from what
// claude answered when it was asked: one may be unless claude said plainly that
// nobody is signed in. an answer that never came counts as a maybe, and so does a
// login that something else masks, such as an apiKeyHelper or a provider: it is
// still stored, and a sign-out is what removes it. a sign-in that did not finish
// and remove both go by it, so they cannot disagree about the same folder.
func loginMayBeIn(st claudecli.AuthStatus, err error) bool {
	return err != nil || st.LoggedIn
}

// holdsClaudeAiLogin says whether the status shows a claude.ai login, which is
// what a sign-out removes. a login that something else supplies, such as an
// apiKeyHelper, is still reported as signed in after one.
func holdsClaudeAiLogin(st claudecli.AuthStatus) bool {
	return st.LoggedIn && st.AuthMethod == "claude.ai"
}

// signedInHere says whether the status is a claude.ai login in this very
// folder, and why not when it is not: why for a caller that acts on it, text
// for one that prints it.
func signedInHere(st claudecli.AuthStatus, folder string) (ok bool, why claudecli.Reason, text string) {
	return st.SignedIn(folder)
}

// logoutFolder signs the folder out through claude. a folder that is not there
// has nothing to sign out of, and is not made just to ask. one that cannot be
// looked at is an error: it may hold a login.
func logoutFolder(folder string) error {
	there, err := folderThere(folder)
	switch {
	case err != nil:
		return err
	case !there:
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), authTimeout)
	defer cancel()
	return claudecli.Logout(ctx, folder)
}

// startLogin builds claude's interactive sign-in for the folder, with the email
// that is expected, if one is recorded. the caller attaches the terminal.
func startLogin(folder, email string) (*exec.Cmd, error) {
	return claudecli.Login(context.Background(), folder, email)
}

// claudeVersion finds claude and asks it for its version.
func claudeVersion() (bin, version string, err error) {
	if bin, err = claudecli.Binary(); err != nil {
		return "", "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), authTimeout)
	defer cancel()
	version, err = claudecli.Version(ctx)
	return bin, version, err
}

// claudeIsOldEnough compares a version with the oldest claude clalt will drive.
func claudeIsOldEnough(version string) (bool, error) {
	cmp, err := claudecli.CompareVersions(version, claudecli.MinVersion)
	return cmp >= 0, err
}

// planLinks lists what syncing the folder with the shared claude home would do,
// as run is going to find it: run makes a shared projects folder that is missing
// before it syncs, and makes says that it will. it writes nothing.
func planLinks(folder string) (steps []profile.Step, makes bool, err error) {
	shared := config.SharedClaudeDir()
	if missingSharedProjects(shared) {
		steps, err = profile.PlanWithFolders(folder, shared, "projects")
		return steps, true, err
	}
	steps, err = profile.Plan(folder, shared)
	return steps, false, err
}

// syncLinks links the folder to the shared claude home.
func syncLinks(folder string) (profile.Report, error) {
	return profile.Sync(folder, config.SharedClaudeDir())
}

// planServers lists the mcp servers that syncing would change. it writes
// nothing.
func planServers(folder string) (profile.MCPChange, error) {
	return profile.PlanMCP(folder, config.SharedClaudeJSONPath())
}

// syncServers mirrors the shared mcp servers into the folder.
func syncServers(folder string) (profile.MCPChange, error) {
	return profile.SyncMCP(folder, config.SharedClaudeJSONPath())
}

// createAccount makes the account for a label, with an id whose folder is free.
func (a *App) createAccount(label string) (account.Account, error) {
	return a.Store.Create(label, folderExists)
}

// relabel changes an account's label. the label that usage reports keep for
// themselves is refused, whichever command asks.
func (a *App) relabel(id, label string) error {
	if err := refuseReservedLabel(label); err != nil {
		return err
	}
	return a.Store.Relabel(id, label)
}
