package cli

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// an id from a v0.1.0 state file was never checked, and rename no longer
// changes ids, so an old id like this one stays. every command that builds a
// folder path from an id validates it first.
const unsafeState = `{"active":"","accounts":{"../../escape":{"addedAt":"2026-01-01T00:00:00Z"}}}`

func noClaudeCalls(t *testing.T, e *testEnv) {
	t.Helper()
	for _, kind := range []string{"login", "logout", "status"} {
		if calls := e.claude.calls(kind); len(calls) != 0 {
			t.Errorf("claude was asked to %s: %v", kind, calls)
		}
	}
	if e.claude.sessionStarted() {
		t.Error("a session was started")
	}
}

func noFolderOutside(t *testing.T, e *testEnv) {
	t.Helper()
	if exists(config.AccountsDir()) || exists(filepath.Join(e.home, ".config", "escape")) {
		t.Error("a folder appeared for an unsafe id")
	}
}

func TestAnUnsafeIdIsRefusedByEveryCommandThatBuildsAPathFromIt(t *testing.T) {
	for _, args := range [][]string{
		{"run", "../../escape"},
		{"login", "../../escape"},
	} {
		t.Run(args[0], func(t *testing.T) {
			e := newEnv(t)
			plantState(t, unsafeState)

			code := e.run(args...)

			if code != 1 || !strings.Contains(e.errOut.String(), "folder name") {
				t.Errorf("exit %d: %s", code, e.errOut.String())
			}
			noClaudeCalls(t, e)
			noFolderOutside(t, e)
		})
	}
}

func TestAnUnsafeIdIsShownButNeverUsedByTheViews(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		e := newEnv(t)
		plantState(t, unsafeState)

		row := listJSON(t, e)[0]

		if row["signedIn"] != nil || !strings.Contains(row["reason"].(string), "folder name") || row["folder"] != "" {
			t.Errorf("row = %v: unknown, with the reason and no path", row)
		}
		noClaudeCalls(t, e)
		noFolderOutside(t, e)
	})
	t.Run("doctor", func(t *testing.T) {
		e := newEnv(t)
		plantState(t, unsafeState)

		code, rep := doctor(t, e)

		rep.want(t, "../../escape", "folder", "fail", "folder name")
		if code != 1 {
			t.Errorf("exit %d, want 1", code)
		}
		noClaudeCalls(t, e)
		noFolderOutside(t, e)
	})
	// a backslash is a separator on windows and a plain character elsewhere, and
	// it is never accepted in an id
	t.Run("whoami", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("a backslash is a path separator here")
		}
		e := newEnv(t)
		plantState(t, `{"active":"","accounts":{"a\\b":{"addedAt":"2026-01-01T00:00:00Z"}}}`)
		t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(config.AccountsDir(), `a\b`))

		got := whoamiJSON(t, e)

		if got["kind"] != "account" || got["signedIn"] != nil || !strings.Contains(got["reason"].(string), "folder name") || got["folder"] != "" {
			t.Errorf("json = %v", got)
		}
		noClaudeCalls(t, e)
	})
}

// logout and remove build no path from an id like that: no folder can exist for
// it in this layout, so there is no login to sign out and nothing to look at.
// remove is the way out, and the refusal of the other commands says so.
func TestAnAccountWithAnUnsafeIdCanStillBeSignedOutAndRemoved(t *testing.T) {
	t.Run("remove", func(t *testing.T) {
		e := newEnv(t)
		plantState(t, unsafeState)

		code := e.run("remove", "../../escape", "--force")

		if code != 0 {
			t.Fatalf("exit %d: %s", code, e.errOut.String())
		}
		if !e.account("../../escape").Meta.Hidden {
			t.Error("the account was not removed")
		}
		out := e.out.String()
		if !strings.Contains(out, "no folder") || !strings.Contains(out, "another name") || strings.Contains(out, "brings it back") {
			t.Errorf("a login cannot bring this account back, and the output should not say it can:\n%s", out)
		}
		noClaudeCalls(t, e)
		noFolderOutside(t, e)
	})
	t.Run("logout", func(t *testing.T) {
		e := newEnv(t)
		plantState(t, unsafeState)

		code := e.run("logout", "../../escape")

		if code != 0 || !strings.Contains(e.out.String(), "nothing to sign out") {
			t.Errorf("exit %d, stdout %q, stderr %q", code, e.out.String(), e.errOut.String())
		}
		noClaudeCalls(t, e)
		noFolderOutside(t, e)
	})
	t.Run("the refusal of login names a command that works", func(t *testing.T) {
		e := newEnv(t)
		plantState(t, unsafeState)

		if code := e.run("login", "../../escape"); code != 1 {
			t.Fatalf("login exit %d: %s", code, e.errOut.String())
		}
		advice := "clalt remove ../../escape"
		if !strings.Contains(e.errOut.String(), advice) {
			t.Fatalf("the refusal does not name %q:\n%s", advice, e.errOut.String())
		}
		e.reset()

		if code := e.run("remove", "../../escape", "--force"); code != 0 {
			t.Errorf("%s: exit %d: %s", advice, code, e.errOut.String())
		}
	})
}

// a machine without a home has no folder for any id, and the message for that
// is not the one about the id
func TestAMissingHomeIsNotBlamedOnTheId(t *testing.T) {
	e := newEnv(t)
	e.claude.signInAs("a@example.com")
	e.signIn("work", "a@example.com")
	acc := e.account("work")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	_, err := folderFor(acc)

	if err == nil || strings.Contains(err.Error(), "folder name") {
		t.Errorf("err = %v: the id is fine, the home is what is missing", err)
	}
}
