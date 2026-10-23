package claudecli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// AuthStatus is the json that `claude auth status --json` prints. keys clalt does not know
// are ignored.
type AuthStatus struct {
	LoggedIn          bool   `json:"loggedIn"`
	AuthMethod        string `json:"authMethod"`
	APIProvider       string `json:"apiProvider"`
	Email             string `json:"email"`
	OrgID             string `json:"orgId"`
	OrgName           string `json:"orgName"`
	SubscriptionType  string `json:"subscriptionType"`
	ConfigDirectory   string `json:"configDirectory"`
	ProjectsDirectory string `json:"projectsDirectory"`
	AnalyticsDisabled bool   `json:"analyticsDisabled"`

	// not part of the json: how the call went. Warning is the first meaningful
	// line claude wrote to stderr, which is where it says it set a corrupt
	// .claude.json aside.
	ExitCode int    `json:"-"`
	Warning  string `json:"-"`
}

// Status asks claude who is signed in to configDir. exit 1 is how claude says
// "signed out" and it still prints json, so that is a normal result; any other
// exit code, or output that is not json, is an error. so is output that does
// not say what it should: see parseStatus. a call without a deadline gets
// authTimeout. callers must not use short timeouts on auth calls: ending claude
// in the middle of a token refresh can sign the folder out.
func Status(ctx context.Context, configDir string) (AuthStatus, error) {
	return status(ctx, configDir, false)
}

// StatusOfShared is Status for the shared claude home. asking who is signed in
// changes nothing, so it is allowed there; login and logout stay refused.
func StatusOfShared(ctx context.Context, configDir string) (AuthStatus, error) {
	return status(ctx, configDir, true)
}

func status(ctx context.Context, configDir string, allowShared bool) (AuthStatus, error) {
	if err := checkFolderFor(configDir, allowShared); err != nil {
		return AuthStatus{}, fmt.Errorf("claude auth status: %w", err)
	}
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()
	if err := ensureVersion(ctx); err != nil {
		return AuthStatus{}, fmt.Errorf("claude auth status: %w", err)
	}
	env := statusEnv(configDir, allowShared)
	res, err := run(ctx, env, "auth", "status", "--json")
	if err != nil {
		return AuthStatus{}, fmt.Errorf("claude auth status: %w", err)
	}
	if res.signal != "" || (res.code != 0 && res.code != 1) {
		return AuthStatus{}, res.failure("auth status", res.ended())
	}
	return parseStatus(res)
}

// statusEnv is the environment of a status call. the default shared home is
// what claude sees with CLAUDE_CONFIG_DIR unset, and on a mac the keychain entry
// of a login is named after that variable, so it stays unset there. it is set
// for an account folder and for a shared home the user chose with the variable.
func statusEnv(configDir string, shared bool) []string {
	if shared {
		if v := os.Getenv(configDirVar); v == "" || !fsutil.SamePath(v, configDir) {
			env, _ := stripAuth(os.Environ())
			return withoutConfigDir(env)
		}
	}
	env, _ := Env(os.Environ(), configDir)
	return env
}

// withoutConfigDir drops CLAUDE_CONFIG_DIR, which stripAuth keeps.
func withoutConfigDir(env []string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if name, _, _ := strings.Cut(kv, "="); envKey(name) != envKey(configDirVar) {
			out = append(out, kv)
		}
	}
	return out
}

// parseStatus reads the json of a finished call. the keys that decide whether a
// folder is signed in must all be present, and claude exits 0 exactly when it is
// signed in, so an exit code that disagrees with loggedIn means the output is
// not what clalt thinks it is.
func parseStatus(res result) (AuthStatus, error) {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(res.stdout, &keys); err != nil || keys == nil {
		return AuthStatus{}, res.failure("auth status", fmt.Sprintf("exit %d with output that is not a json object", res.code))
	}
	for _, k := range []string{"loggedIn", "authMethod", "configDirectory"} {
		if _, ok := keys[k]; !ok {
			return AuthStatus{}, res.failure("auth status", "the json has no "+k)
		}
	}
	var st AuthStatus
	if err := json.Unmarshal(res.stdout, &st); err != nil {
		return AuthStatus{}, res.failure("auth status", "the json has a key of the wrong type")
	}
	if (res.code == 0) != st.LoggedIn {
		return AuthStatus{}, res.failure("auth status", fmt.Sprintf("exit %d but loggedIn is %v", res.code, st.LoggedIn))
	}
	st.ExitCode = res.code
	st.Warning = meaningfulLine(res.stderr)
	return st, nil
}

// Reason says why a status does not count as signed in, so a caller can act on
// it without reading the text.
type Reason int

// ReasonNone, ReasonNotSignedIn, ReasonOtherAuthMethod, ReasonOtherProvider,
// ReasonNoEmail, ReasonNoFolder and ReasonOtherFolder are the reasons SignedIn
// gives, in the order it checks them. ReasonNone means the status does count as
// signed in.
const (
	ReasonNone Reason = iota
	ReasonNotSignedIn
	ReasonOtherAuthMethod
	ReasonOtherProvider
	ReasonNoEmail
	ReasonNoFolder
	ReasonOtherFolder
)

// String gives the name of the reason, for messages and test output.
func (r Reason) String() string {
	switch r {
	case ReasonNone:
		return "none"
	case ReasonNotSignedIn:
		return "not signed in"
	case ReasonOtherAuthMethod:
		return "other auth method"
	case ReasonOtherProvider:
		return "other provider"
	case ReasonNoEmail:
		return "no email"
	case ReasonNoFolder:
		return "no folder reported"
	case ReasonOtherFolder:
		return "other folder"
	}
	return "unknown"
}

// SignedIn reports whether the status counts as signed in for configDir: a
// claude.ai login with an email, in the first party provider, found in that very
// folder. anything else (an api key helper from shared settings, a third party
// provider, a credential that has no email because it expired, a login from
// another folder) would make a session run as the wrong identity or as nobody,
// so why says which it is and text says it in words.
func (s AuthStatus) SignedIn(configDir string) (ok bool, why Reason, text string) {
	switch {
	case !s.LoggedIn:
		return false, ReasonNotSignedIn, "not signed in"
	case s.AuthMethod != "claude.ai":
		method := s.AuthMethod
		if method == "" {
			method = "an unknown method"
		}
		return false, ReasonOtherAuthMethod, "signed in with " + method + ", not a claude.ai account"
	case s.APIProvider != "" && s.APIProvider != "firstParty":
		return false, ReasonOtherProvider, "uses the " + s.APIProvider + " provider, not claude.ai directly"
	case s.Email == "":
		return false, ReasonNoEmail, "claude reports a login without an email, which means it expired or is incomplete"
	case s.ConfigDirectory == "":
		return false, ReasonNoFolder, "claude did not report which folder it checked"
	case !fsutil.SamePath(s.ConfigDirectory, configDir):
		return false, ReasonOtherFolder, "claude checked a different folder: " + s.ConfigDirectory
	}
	return true, ReasonNone, ""
}
