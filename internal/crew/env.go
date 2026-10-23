package crew

import (
	"sort"
	"strconv"
	"strings"

	"github.com/dhitalkamal/clalt/internal/claudecli"
)

// the variables GatewayEnv sets itself. an inherited one of the same name is
// replaced, which is expected, so it is not reported as removed.
var setByClalt = map[string]bool{
	"KIROCREW_HOME": true, "KIRO_HOME": true, "KIROCREW_WORKSPACE": true, "KIROCREW_PORT": true,
	"KIROCREW_SKIP_MODEL_DOWNLOAD": true, "KIROCREW_BIND": true, "CLAUDE_CODE_EXECUTABLE": true,
}

// channelPrefixes start the variables that hold credentials of the channels
// Kiro Crew can connect to. a gateway of an account has no business with the
// ones in the shell of whoever starts it.
var channelPrefixes = []string{"SLACK_", "TELEGRAM_", "DISCORD_", "WEBEX_", "WECOM_", "FEISHU_", "JIRA_"}

// GatewayEnv builds the environment of a gateway from base, which is left as it
// is. it has no variable that would override the account's login (the same list
// claudecli removes before every claude call), none that points at another Kiro
// Crew home or changes how it approves tool calls, and no channel credential.
// it sets the account's folder, the gateway's own homes and port, the loopback
// address to bind, the claude that the adapter runs, and no download of the
// embedding model: that is 640 MB per account. removed holds the names of what
// was taken out, never their values.
func GatewayEnv(base []string, folder, claudeBin string, p Paths, port int) (env, removed []string) {
	env, auth := claudecli.Env(base, folder)
	env, kiro := dropKiro(env)
	env = append(env,
		"KIROCREW_HOME="+p.Home,
		"KIRO_HOME="+p.KiroHome,
		"KIROCREW_WORKSPACE="+p.Workspace,
		"KIROCREW_PORT="+strconv.Itoa(port),
		"KIROCREW_BIND=127.0.0.1",
		"KIROCREW_SKIP_MODEL_DOWNLOAD=1",
		"CLAUDE_CODE_EXECUTABLE="+claudeBin,
	)
	removed = append(auth, kiro...)
	sort.Strings(removed)
	return env, removed
}

// ProbeEnv is the environment for asking kirocrew its version: nothing that
// points at another home, and the home it is given in case it ever touches one.
func ProbeEnv(base []string, home string) []string {
	env, _ := dropKiro(base)
	return append(env, "KIROCREW_HOME="+home)
}

// dropKiro removes every KIROCREW_ variable, KIRO_HOME, CLAUDE_CODE_EXECUTABLE
// and the channel credentials, and reports the names of the ones that
// GatewayEnv does not set again.
func dropKiro(base []string) (env, dropped []string) {
	seen := map[string]bool{}
	for _, kv := range base {
		name, _, _ := strings.Cut(kv, "=")
		if !dropsName(name) {
			env = append(env, kv)
			continue
		}
		if !setByClalt[name] && !seen[name] {
			seen[name] = true
			dropped = append(dropped, name)
		}
	}
	return env, dropped
}

func dropsName(name string) bool {
	if strings.HasPrefix(name, "KIROCREW_") || name == "KIRO_HOME" || name == "CLAUDE_CODE_EXECUTABLE" {
		return true
	}
	for _, pre := range channelPrefixes {
		if strings.HasPrefix(name, pre) {
			return true
		}
	}
	return false
}
