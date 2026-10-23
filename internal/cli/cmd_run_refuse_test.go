package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// every refusal names the command that fixes it, starts no session and does not
// count as a use of the account.
func TestRunRefusals(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *testEnv)
		label string
		want  []string
	}{
		{
			name:  "an account that does not exist",
			setup: func(e *testEnv) {},
			label: "nobody",
			want:  []string{`no account called "nobody"`, "clalt login nobody"},
		},
		{
			name: "an account that was removed",
			setup: func(e *testEnv) {
				e.signIn("work", "a@example.com")
				if err := e.app.Store.Hide(e.account("work").Name); err != nil {
					t.Fatal(err)
				}
			},
			label: "work",
			want:  []string{`"work" was removed`, "clalt login work"},
		},
		{
			name: "an account that signed out",
			setup: func(e *testEnv) {
				e.signIn("work", "a@example.com")
				e.claude.signOut(e.folder("work"))
			},
			label: "work",
			want:  []string{"not signed in", "clalt login work"},
		},
		{
			name: "a folder signed in as somebody else",
			setup: func(e *testEnv) {
				e.signIn("work", "a@example.com")
				e.claude.plantLogin(e.folder("work"), "other@example.com")
			},
			label: "work",
			want:  []string{"different account than the one recorded", "clalt login work"},
		},
		{
			name: "a signed in folder with no recorded email",
			setup: func(e *testEnv) {
				acc, err := e.app.createAccount("work")
				if err != nil {
					t.Fatal(err)
				}
				e.claude.plantLogin(e.folder(acc.Label()), "a@example.com")
			},
			label: "work",
			want:  []string{"no sign-in is recorded", "clalt login work"},
		},
		{
			name: "a folder that falls back to an api key helper",
			setup: func(e *testEnv) {
				e.signIn("work", "a@example.com")
				e.claude.set("API_KEY_HELPER", "1")
			},
			label: "work",
			want:  []string{"api_key_helper", "overrides", "clalt doctor"},
		},
		{
			name: "a status check that breaks",
			setup: func(e *testEnv) {
				e.signIn("work", "a@example.com")
				e.claude.set("STATUS_BROKEN", "1")
			},
			label: "work",
			want:  []string{"keychain is locked"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			c.setup(e)
			e.reset()

			code := e.run("run", c.label)

			msg := e.errOut.String()
			if code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
			for _, want := range c.want {
				if !strings.Contains(msg, want) {
					t.Errorf("stderr is missing %q:\n%s", want, msg)
				}
			}
			if e.claude.sessionStarted() {
				t.Error("a session was started")
			}
			if e.hasAccount(c.label) {
				if got := e.account(c.label).Meta.LastUsedAt; !got.IsZero() {
					t.Errorf("lastUsedAt = %v, a refused run is not a use", got)
				}
			}
		})
	}
}

func TestRunNeverCreatesAFolderForAnUnknownName(t *testing.T) {
	e := newEnv(t)

	code := e.run("run", "../../escape")

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if exists(config.AccountsDir()) || exists(filepath.Join(e.home, ".config", "escape")) {
		t.Error("run created a folder for a name that is no account")
	}
}

func TestRunMistakesAreUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"run"}, {"run", "--"}, {"run", "-x", "work"}} {
		e := newEnv(t)
		code := e.run(args...)
		if code != 2 || !strings.Contains(e.errOut.String(), "usage: clalt run <name> [-- claude args]") {
			t.Errorf("%v: exit %d, stderr %q", args, code, e.errOut.String())
		}
	}
}
