package cli

import (
	"strings"
	"testing"
)

// remove signs the folder out unless claude says plainly that nobody is signed
// in. a login that something else masks (an apiKeyHelper, a provider) or that
// claude reports in an odd way can still be stored in the folder, and a hidden
// account is one nobody lists any more.
func TestRemoveSignsOutAFolderWhoseLoginClaudeReportsInAnyWayButNone(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *testEnv, folder string)
	}{
		{"an api key helper masks it", func(e *testEnv, _ string) { e.claude.set("API_KEY_HELPER", "1") }},
		{"a provider masks it", func(e *testEnv, _ string) { e.claude.set("PROVIDER", "bedrock") }},
		{"claude reports it without an email", func(e *testEnv, folder string) { e.claude.plantLogin(folder, "") }},
		{"claude reports it for another folder", func(e *testEnv, _ string) { e.claude.set("STATUS_FOLDER", "/somewhere/else") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.signIn("work", "w@example.com")
			folder := e.folder("work")
			c.setup(e, folder)
			before := len(e.claude.calls("logout"))

			code := e.run("remove", "work", "--force")

			if code != 0 {
				t.Fatalf("exit %d: %s", code, e.errOut.String())
			}
			if got := len(e.claude.calls("logout")); got != before+1 {
				t.Errorf("logout calls %d then %d: the stored login was left in a hidden folder", before, got)
			}
			if e.claude.signedIn(folder) {
				t.Error("a login is still in the folder of a removed account")
			}
			if !e.account("work").Meta.Hidden {
				t.Error("the account was not removed")
			}
		})
	}
}

// and when that sign-out fails the account stays, as it does for any other login
func TestRemoveThatCannotSignOutAMaskedLoginStopsWithoutHiding(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.set("API_KEY_HELPER", "1")
	e.claude.set("LOGOUT_FAILS", "1")

	code := e.run("remove", "work", "--force")

	if code != 1 || !strings.Contains(e.errOut.String(), "could not reach the keychain") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
	if e.account("work").Meta.Hidden {
		t.Error("the account was hidden while its login is still there")
	}
}
