package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/claudecli"
	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// loginWarnWindow is how far ahead doctor warns that an account has to sign in
// again.
const loginWarnWindow = 3 * 24 * time.Hour

// claudeCheck says whether there is a claude that clalt can drive. usable is
// false when there is none, which the checks that ask claude take into account.
func claudeCheck() (res checkResult, usable bool) {
	bin, version, err := claudeVersion()
	if err != nil {
		return result("claude", "version", statusFail, "%v", err), false
	}
	oldEnough, err := claudeIsOldEnough(version)
	switch {
	case err != nil:
		return result("claude", "version", statusFail, "%v", err), false
	case !oldEnough:
		return result("claude", "version", statusFail,
			"claude %s is older than %s, the oldest clalt will drive (it can be signed out in the middle of a refresh). update claude",
			version, claudecli.MinVersion), false
	}
	return result("claude", "version", statusOK, "claude %s at %s", version, bin), true
}

// answer is what claude said about one account folder.
type answer struct {
	status claudecli.AuthStatus
	err    error
}

// accountChecks checks every visible account. the questions to claude run side
// by side, the rest is reading files.
func (a *App) accountChecks(claudeUsable bool) ([]checkResult, error) {
	accts, err := a.visibleAccounts()
	if err != nil {
		return nil, err
	}
	answers := make([]answer, len(accts))
	if claudeUsable {
		parallelDo(len(accts), statusWorkers, func(i int) {
			if folder, err := folderFor(accts[i]); err == nil {
				answers[i].status, answers[i].err = liveStatus(folder)
			}
		})
	}
	var out []checkResult
	for i, acc := range accts {
		out = append(out, a.accountCheck(acc, answers[i], claudeUsable)...)
	}
	return append(out, a.removedChecks(claudeUsable)...), nil
}

func (a *App) accountCheck(acc account.Account, ans answer, claudeUsable bool) []checkResult {
	folder, err := folderFor(acc)
	if err != nil {
		return []checkResult{result(acc.Label(), "folder", statusFail, "%v", err)}
	}
	out := a.folderChecks(acc, folder)
	return append(out, a.signInChecks(acc, folder, ans, claudeUsable)...)
}

// folderChecks look at the folder itself and what is in it. an account that
// shares its folder with another one gets only that, since links and servers
// there are the other account's as much as its own.
func (a *App) folderChecks(acc account.Account, folder string) []checkResult {
	label := acc.Label()
	others, err := a.foldersSharedWith(acc)
	switch {
	case err != nil:
		return []checkResult{result(label, "folder", statusWarn, "could not check whether the folder is shared: %v", err)}
	case len(others) > 0:
		return []checkResult{result(label, "folder", statusFail, "%s", sharedFolderMessage(acc, others))}
	}
	switch there, err := folderThere(folder); {
	case err != nil:
		return []checkResult{result(label, "folder", statusWarn, "could not look at the folder: %v", err)}
	case !there:
		return []checkResult{result(label, "folder", statusWarn, "the folder is missing. sign in with: clalt login %s", label)}
	}
	return []checkResult{
		result(label, "folder", statusOK, "the folder is there: %s", folder),
		linksCheck(label, folder),
		mcpCheck(label, folder),
	}
}

func (a *App) signInChecks(acc account.Account, folder string, ans answer, claudeUsable bool) []checkResult {
	label, meta := acc.Label(), acc.Meta
	var out []checkResult
	signedIn := false
	switch {
	case !claudeUsable:
		out = append(out, result(label, "signedIn", statusWarn, "not checked, claude is not usable (see the claude check)"))
	case ans.err != nil:
		out = append(out, result(label, "signedIn", statusWarn, "could not ask claude: %v", ans.err))
	default:
		if ok, why, text := signedInHere(ans.status, folder); !ok {
			out = append(out, result(label, "signedIn", statusWarn, "%s. %s", text, loginAdvice(why, label)))
		} else {
			signedIn = true
			out = append(out, result(label, "signedIn", statusOK, "signed in"))
		}
	}
	if signedIn {
		switch {
		case meta.Email == "":
			out = append(out, result(label, "identity", statusWarn,
				"claude has a login in this folder but clalt has no record of whose it is. run: clalt login %s", label))
		case !sameEmail(ans.status.Email, meta.Email):
			out = append(out, result(label, "identity", statusFail,
				"signed in as a different account than the one recorded, %s. run: clalt login %s", meta.Email, label))
		default:
			out = append(out, result(label, "identity", statusOK, "signed in as %s", meta.Email))
		}
	}
	// a failed call has no status, its stderr is already in the error above
	if note := ans.status.Warning; note != "" {
		out = append(out, result(label, "claudeWarning", statusWarn, "claude said: %s", note))
	}
	if !meta.SignedInAt.IsZero() {
		out = append(out, a.loginDueCheck(label, meta.SignedInAt))
	}
	return out
}

func (a *App) loginDueCheck(label string, signedInAt time.Time) checkResult {
	due := loginDue(signedInAt)
	left := due.Sub(a.clock())
	switch {
	case left < 0:
		return result(label, "loginDue", statusWarn, "the sign-in was due on %s. sign in again with: clalt login %s", dateOf(due), label)
	case left < loginWarnWindow:
		return result(label, "loginDue", statusWarn, "sign in again by %s (%s left). run: clalt login %s", dateOf(due), timeLeft(left), label)
	}
	return result(label, "loginDue", statusOK, "sign in again by %s", dateOf(due))
}

func timeLeft(d time.Duration) string {
	if d < 24*time.Hour {
		return "less than a day"
	}
	return count(int(d.Hours()/24), "day")
}

// envCheck names the auth variables set in this shell. claude would use them
// over the account's login, clalt takes them out before it runs claude.
func envCheck() checkResult {
	_, set := claudecli.Env(os.Environ(), "")
	if len(set) == 0 {
		return result("shell", "authEnv", statusOK, "no variable that overrides a login is set")
	}
	return result("shell", "authEnv", statusWarn,
		"%s set in this shell. clalt does not pass them to claude, but plain claude would use them", strings.Join(set, ", "))
}

// settingsCheck looks in the shared settings for what would override the login
// of every account: an apiKeyHelper, or an env block that sets an auth
// variable. it reports names only, never what they are set to.
func settingsCheck() checkResult {
	shared := config.SharedClaudeDir()
	if shared == "" {
		return result("settings", "authSettings", statusWarn, "not checked: %s", noHomeKnown)
	}
	var found, unreadable []string
	for _, name := range []string{"settings.json", "settings.local.json"} {
		raw, err := fsutil.ReadFile(filepath.Join(shared, name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		var s struct {
			APIKeyHelper json.RawMessage            `json:"apiKeyHelper"`
			Env          map[string]json.RawMessage `json:"env"`
		}
		if err != nil || json.Unmarshal(raw, &s) != nil {
			unreadable = append(unreadable, name)
			continue
		}
		if h := strings.TrimSpace(string(s.APIKeyHelper)); h != "" && h != "null" && h != `""` {
			found = append(found, name+" sets apiKeyHelper")
		}
		var vars []string
		for key := range s.Env {
			if claudecli.IsAuthEnvVar(key) {
				vars = append(vars, key)
			}
		}
		sort.Strings(vars)
		if len(vars) > 0 {
			found = append(found, fmt.Sprintf("%s sets %s in env", name, strings.Join(vars, ", ")))
		}
	}
	if len(found) == 0 && len(unreadable) == 0 {
		return result("settings", "authSettings", statusOK, "no apiKeyHelper or auth variable in the shared settings")
	}
	var parts []string
	if len(found) > 0 {
		parts = append(parts, strings.Join(found, "; ")+". that overrides the login of every account")
	}
	if len(unreadable) > 0 {
		parts = append(parts, strings.Join(unreadable, ", ")+" could not be read as json")
	}
	return result("settings", "authSettings", statusWarn, "%s", strings.Join(parts, ". "))
}
