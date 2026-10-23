// Package claudecli is the only place clalt runs claude's own auth commands.
package claudecli

import (
	"runtime"
	"sort"
	"strings"
)

// every one of these outranks a subscription login in claude's credential
// precedence, so any of them set in the shell would override the login of the
// account folder clalt points claude at.
var authEnvVars = []string{
	"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_CODE_OAUTH_REFRESH_TOKEN", "CLAUDE_CODE_OAUTH_SCOPES",
	"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_PROFILE",
	"ANTHROPIC_FEDERATION_RULE_ID", "ANTHROPIC_ORGANIZATION_ID",
	"CLAUDE_SECURESTORAGE_CONFIG_DIR", "CLAUDE_CODE_USE_BEDROCK",
	"CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY",
	// provider and credential switches from the env-vars docs, and the two that
	// could send the account's token to another host
	"CLAUDE_CODE_USE_MANTLE", "CLAUDE_CODE_USE_ANTHROPIC_AWS", "ANTHROPIC_AWS_API_KEY",
	"ANTHROPIC_BASE_URL", "ANTHROPIC_CUSTOM_HEADERS",
}

const configDirVar = "CLAUDE_CONFIG_DIR"

// caseFold is a variable so tests can exercise the Windows rule anywhere.
var caseFold = runtime.GOOS == "windows"

func envKey(name string) string {
	if caseFold {
		return strings.ToUpper(name)
	}
	return name
}

// Env returns base without the auth variables and with CLAUDE_CONFIG_DIR set to
// configDir. removed holds the names that were stripped, sorted and without
// values, so the caller can tell the user. the config dir is replaced, not
// reported: it is expected to differ.
func Env(base []string, configDir string) (env, removed []string) {
	env, removed = strip(base, true)
	return append(env, configDirVar+"="+configDir), removed
}

// stripAuth is Env without a config dir: for a call that is not about any
// account folder, such as asking for the version.
func stripAuth(base []string) (env, removed []string) { return strip(base, false) }

func strip(base []string, dropConfigDir bool) (env, removed []string) {
	canon := map[string]string{}
	for _, n := range authEnvVars {
		canon[envKey(n)] = n
	}
	gone := map[string]bool{}
	for _, kv := range base {
		name, _, _ := strings.Cut(kv, "=")
		key := envKey(name)
		switch {
		case dropConfigDir && key == envKey(configDirVar):
			// replaced by the caller
		case canon[key] != "":
			gone[canon[key]] = true
		default:
			env = append(env, kv)
		}
	}
	for n := range gone {
		removed = append(removed, n)
	}
	sort.Strings(removed)
	return env, removed
}
