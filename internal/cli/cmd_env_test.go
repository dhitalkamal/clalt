package cli

import (
	"strings"
	"testing"
)

// the variables that would override an account's login are taken out of every
// call to claude, not only of the session: a login, a status check and a
// sign-out each run with the account's folder as CLAUDE_CONFIG_DIR and none of
// the auth variables of the shell.
func TestEveryCallToClaudeRunsInTheAccountFolderWithoutTheAuthVariables(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-secret")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "tok-test-secret")
	e.signIn("work", "a@example.com")
	if code := e.run("logout", "work"); code != 0 {
		t.Fatalf("logout exited %d: %s", code, e.errOut.String())
	}
	folder := e.folder("work")

	for _, kind := range []string{"login", "status", "logout"} {
		env := e.claude.callEnv(kind)
		for name, value := range env {
			if strings.HasPrefix(name, "ANTHROPIC_") || strings.HasPrefix(name, "CLAUDE_CODE_OAUTH") || strings.Contains(value, "-test-secret") {
				t.Errorf("the %s call saw %s=%s", kind, name, value)
			}
		}
		if got := env["CLAUDE_CONFIG_DIR"]; got != folder {
			t.Errorf("the %s call ran with CLAUDE_CONFIG_DIR=%q, want %q", kind, got, folder)
		}
	}
}

// what claude reports is the whole identity: an organization from an earlier
// sign-in does not outlive one that has none.
func TestASignInWithoutAnOrganizationClearsTheOneFromBefore(t *testing.T) {
	e := newEnv(t)
	e.claude.set("ORG", "Acme")
	e.signIn("work", "a@example.com")
	if got := e.account("work").Meta.Org; got != "Acme" {
		t.Fatalf("setup: org = %q", got)
	}
	e.claude.set("ORG", "")
	e.claude.set("NO_ORG", "1")

	if code := e.run("login", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := e.account("work").Meta.Org; got != "" {
		t.Errorf("org = %q, want none", got)
	}
	if out := e.out.String(); !strings.Contains(out, "work is signed in as a@example.com (max)\n") {
		t.Errorf("output:\n%s", out)
	}
}
