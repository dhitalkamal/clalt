package claudecli

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// the list from docs/plans/thin-profiles.md, typed out again on purpose so a
// change to authEnvVars has to be a change to the plan too.
var planAuthVars = []string{
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

func TestEnvStripsEveryVarOnThePlanList(t *testing.T) {
	var base []string
	for _, n := range planAuthVars {
		base = append(base, n+"=value-of-"+n)
	}
	base = append(base, "PATH=/usr/bin")

	env, removed := Env(base, "/acct/a")

	for _, kv := range env {
		for _, n := range planAuthVars {
			if strings.HasPrefix(kv, n+"=") {
				t.Errorf("%s survived in the environment", n)
			}
		}
	}
	if want := sortedCopy(planAuthVars); !reflect.DeepEqual(removed, want) {
		t.Errorf("removed = %v, want %v", removed, want)
	}
}

func TestEnvPointsAtTheFolderOnceAndKeepsTheRest(t *testing.T) {
	base := []string{
		"PATH=/usr/bin", "CLAUDE_CONFIG_DIR=/somewhere/else", "HOME=/h",
		"ANTHROPIC_API_KEY_EXTRA=keep-me", "NOT_AN_ASSIGNMENT", "CLAUDE_CONFIG_DIR=/twice",
	}

	env, removed := Env(base, "/acct/a")

	want := []string{"PATH=/usr/bin", "HOME=/h", "ANTHROPIC_API_KEY_EXTRA=keep-me", "NOT_AN_ASSIGNMENT", "CLAUDE_CONFIG_DIR=/acct/a"}
	if !reflect.DeepEqual(env, want) {
		t.Errorf("env = %v, want %v", env, want)
	}
	// the config dir is replaced, not an override clalt strips and announces
	if len(removed) != 0 {
		t.Errorf("removed = %v, want none", removed)
	}
}

func TestEnvReportsNamesOnceAndNeverValues(t *testing.T) {
	base := []string{
		"ANTHROPIC_API_KEY=sk-secret-1", "CLAUDE_CODE_OAUTH_TOKEN=tok-secret",
		"ANTHROPIC_API_KEY=sk-secret-2", "CLAUDE_CODE_USE_BEDROCK=",
	}

	_, removed := Env(base, "/acct/a")

	want := []string{"ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_CODE_USE_BEDROCK"}
	if !reflect.DeepEqual(removed, want) {
		t.Errorf("removed = %v, want %v", removed, want)
	}
	for _, r := range removed {
		if strings.Contains(r, "secret") || strings.Contains(r, "=") {
			t.Errorf("removed entry %q carries a value", r)
		}
	}
}

func TestEnvDoesNotTouchItsInput(t *testing.T) {
	base := []string{"ANTHROPIC_API_KEY=x", "PATH=/p"}
	Env(base, "/acct/a")
	if !reflect.DeepEqual(base, []string{"ANTHROPIC_API_KEY=x", "PATH=/p"}) {
		t.Errorf("base was modified: %v", base)
	}
}

func TestEnvIgnoresCaseWhereTheOSDoes(t *testing.T) {
	old := caseFold
	caseFold = true
	t.Cleanup(func() { caseFold = old })

	env, removed := Env([]string{"Anthropic_Api_Key=x", "claude_config_dir=/old", "Path=/p"}, "/acct/a")

	if want := []string{"Path=/p", "CLAUDE_CONFIG_DIR=/acct/a"}; !reflect.DeepEqual(env, want) {
		t.Errorf("env = %v, want %v", env, want)
	}
	// the name is the plan's spelling, whatever the shell used
	if want := []string{"ANTHROPIC_API_KEY"}; !reflect.DeepEqual(removed, want) {
		t.Errorf("removed = %v, want %v", removed, want)
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
