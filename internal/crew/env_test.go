package crew

import (
	"slices"
	"strings"
	"testing"
)

// valueOf finds the one entry of name, and fails the test when there are none or
// several.
func valueOf(t *testing.T, env []string, name string) string {
	t.Helper()
	var found []string
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == name {
			found = append(found, v)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s has %d entries: %v", name, len(found), found)
	}
	return found[0]
}

func has(env []string, name string) bool {
	for _, kv := range env {
		if k, _, _ := strings.Cut(kv, "="); k == name {
			return true
		}
	}
	return false
}

var ambient = []string{
	"PATH=/usr/bin:/opt/homebrew/bin", "HOME=/home/me", "LANG=en_US.UTF-8",
	"ANTHROPIC_API_KEY=sk-ant-very-secret", "CLAUDE_CODE_OAUTH_TOKEN=oauth-secret", "ANTHROPIC_BASE_URL=https://elsewhere",
	"CLAUDE_CONFIG_DIR=/home/me/.claude",
	"KIROCREW_HOME=/home/me/.kiro/crew", "KIROCREW_PORT=5476", "KIROCREW_CC_PERMISSION_MODE=auto", "KIRO_HOME=/home/me/.kiro",
}

func TestAGatewayRunsInItsOwnAccountsFolderAndItsOwnHome(t *testing.T) {
	isolate(t)
	p, _ := PathsFor("work")
	env, _ := GatewayEnv(ambient, "/accounts/work", "/bin/claude", p, 5501)
	for name, want := range map[string]string{
		"CLAUDE_CONFIG_DIR": "/accounts/work", "KIROCREW_HOME": p.Home, "KIRO_HOME": p.KiroHome,
		"KIROCREW_WORKSPACE": p.Workspace, "KIROCREW_PORT": "5501", "KIROCREW_BIND": "127.0.0.1", "CLAUDE_CODE_EXECUTABLE": "/bin/claude", "KIROCREW_SKIP_MODEL_DOWNLOAD": "1",
		"PATH": "/usr/bin:/opt/homebrew/bin", "HOME": "/home/me", "LANG": "en_US.UTF-8",
	} {
		if got := valueOf(t, env, name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestNothingThatOverridesTheLoginOrPointsAtAnotherHomeIsPassedOn(t *testing.T) {
	isolate(t)
	p, _ := PathsFor("work")
	env, removed := GatewayEnv(ambient, "/accounts/work", "/bin/claude", p, 5501)
	for _, name := range []string{"ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_BASE_URL", "KIROCREW_CC_PERMISSION_MODE"} {
		if has(env, name) {
			t.Errorf("%s reached the gateway", name)
		}
	}
	want := []string{"ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "CLAUDE_CODE_OAUTH_TOKEN", "KIROCREW_CC_PERMISSION_MODE"}
	if !slices.Equal(removed, want) {
		t.Errorf("removed = %v, want %v (the ones that are replaced are not reported)", removed, want)
	}
	for _, name := range removed {
		if strings.Contains(name, "=") || strings.Contains(name, "secret") {
			t.Errorf("removed holds more than a name: %q", name)
		}
	}
}

func TestChannelCredentialsAreNotPassedOnAndOnlyTheirNamesAreReported(t *testing.T) {
	isolate(t)
	p, _ := PathsFor("work")
	base := append(slices.Clone(ambient),
		"SLACK_BOT_TOKEN=xoxb-secret", "TELEGRAM_BOT_TOKEN=tg-secret", "DISCORD_TOKEN=dc-secret", "WEBEX_ACCESS_TOKEN=wx-secret",
		"WECOM_SECRET=wc-secret", "FEISHU_APP_SECRET=fs-secret", "JIRA_API_TOKEN=jr-secret", "SLACKISH=keep", "MY_SLACK_X=keep")
	env, removed := GatewayEnv(base, "/accounts/work", "/bin/claude", p, 5501)
	for _, name := range []string{"SLACK_BOT_TOKEN", "TELEGRAM_BOT_TOKEN", "DISCORD_TOKEN", "WEBEX_ACCESS_TOKEN", "WECOM_SECRET", "FEISHU_APP_SECRET", "JIRA_API_TOKEN"} {
		if has(env, name) {
			t.Errorf("%s reached the gateway", name)
		}
		if !slices.Contains(removed, name) {
			t.Errorf("%s was not reported", name)
		}
	}
	for _, name := range []string{"SLACKISH", "MY_SLACK_X"} {
		if !has(env, name) {
			t.Errorf("%s was taken out, it is not a channel credential", name)
		}
	}
	if strings.Contains(strings.Join(removed, " "), "secret") {
		t.Errorf("a value in %v", removed)
	}
}

func TestBuildingTheEnvironmentLeavesTheCallersAlone(t *testing.T) {
	isolate(t)
	p, _ := PathsFor("work")
	base := slices.Clone(ambient)
	GatewayEnv(base, "/accounts/work", "/bin/claude", p, 5501)
	if !slices.Equal(base, ambient) {
		t.Errorf("the caller's environment changed: %v", base)
	}
}

func TestTheVersionProbeSeesNoOtherHomeAndNoLogin(t *testing.T) {
	env := ProbeEnv(ambient, "/probe/home")
	if got := valueOf(t, env, "KIROCREW_HOME"); got != "/probe/home" {
		t.Errorf("KIROCREW_HOME = %q", got)
	}
	for _, name := range []string{"KIRO_HOME", "KIROCREW_PORT", "KIROCREW_CC_PERMISSION_MODE"} {
		if has(env, name) {
			t.Errorf("%s reached the probe", name)
		}
	}
	if got := valueOf(t, env, "PATH"); got != "/usr/bin:/opt/homebrew/bin" {
		t.Errorf("PATH = %q", got)
	}
}
