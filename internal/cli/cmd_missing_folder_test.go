package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// a v0.1.0 account has no folder until it signs in, and claude would create the
// folder it is asked about. clalt only asks about a folder that exists, so a
// view cannot make one, and a folder that is not there has no login to show.
func legacyWithoutAFolder(t *testing.T) *testEnv {
	t.Helper()
	e := newEnv(t)
	plantState(t, `{"active":"work","accounts":{"work":{"email":"w@example.com","plan":"max","addedAt":"2026-08-01T10:00:00Z"}}}`)
	return e
}

func TestViewsDoNotAskClaudeAboutAFolderThatIsNotThere(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		e := legacyWithoutAFolder(t)
		row := listJSON(t, e)[0]
		if row["signedIn"] != false || row["reason"] != "not signed in" {
			t.Errorf("row = %v", row)
		}
		noClaudeCalls(t, e)
	})
	t.Run("whoami", func(t *testing.T) {
		e := legacyWithoutAFolder(t)
		t.Setenv("CLAUDE_CONFIG_DIR", accountDir(t, "work"))
		got := whoamiJSON(t, e)
		if got["signedIn"] != false || got["reason"] != "not signed in" {
			t.Errorf("json = %v", got)
		}
		noClaudeCalls(t, e)
	})
	t.Run("doctor", func(t *testing.T) {
		e := legacyWithoutAFolder(t)
		_, rep := doctor(t, e)
		rep.want(t, "work", "folder", "warn", "clalt login work")
		rep.want(t, "work", "signedIn", "warn", "not signed in")
		noClaudeCalls(t, e)
		if exists(accountDir(t, "work")) {
			t.Error("doctor made the folder")
		}
	})
	t.Run("a view never makes the folder", func(t *testing.T) {
		e := legacyWithoutAFolder(t)
		e.run("list")
		e.run("whoami")
		if exists(config.AccountsDir()) {
			t.Error("a view made the accounts folder")
		}
	})
}

func TestLogoutAndRemoveOfAnAccountWithoutAFolderNeedNoClaude(t *testing.T) {
	t.Run("logout", func(t *testing.T) {
		e := legacyWithoutAFolder(t)

		if code := e.run("logout", "work"); code != 0 {
			t.Fatalf("exit %d: %s", code, e.errOut.String())
		}

		noClaudeCalls(t, e)
		if !strings.Contains(e.out.String(), "work is signed out") {
			t.Errorf("output:\n%s", e.out.String())
		}
	})
	t.Run("remove", func(t *testing.T) {
		e := legacyWithoutAFolder(t)

		if code := e.run("remove", "work", "--force"); code != 0 {
			t.Fatalf("exit %d: %s", code, e.errOut.String())
		}

		noClaudeCalls(t, e)
		if !e.account("work").Meta.Hidden {
			t.Error("the account was not removed")
		}
		if exists(accountDir(t, "work")) {
			t.Error("remove made the folder")
		}
	})
}

// run and login make the folder first, because they are about to use it, and
// only then ask claude.
func TestRunAndLoginMakeTheFolderBeforeTheyAskClaude(t *testing.T) {
	for _, cmd := range []string{"run", "login"} {
		t.Run(cmd, func(t *testing.T) {
			e := legacyWithoutAFolder(t)
			e.claude.signInAs("w@example.com")

			e.run(cmd, "work")

			if !exists(accountDir(t, "work")) {
				t.Error("the folder was not made")
			}
			calls := e.claude.calls("status")
			if len(calls) == 0 || filepath.Base(calls[0]) != "work" {
				t.Errorf("claude was not asked about the folder: %v", calls)
			}
		})
	}
}
