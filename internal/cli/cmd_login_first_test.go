package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func labelsInList(t *testing.T, e *testEnv) []string {
	t.Helper()
	e.reset()
	var labels []string
	for _, row := range listJSON(t, e) {
		labels = append(labels, row["label"].(string))
	}
	return labels
}

// an account that this very login made, and that nobody signed in to, is not
// left in the list. it is hidden, not deleted: its folder is not orphaned, and
// the next login under the same name reuses it.
func TestAFirstLoginThatFailsLeavesNoAccountInTheList(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *testEnv)
		fix   func(e *testEnv)
	}{
		{"claude exits non-zero",
			func(e *testEnv) { e.claude.set("LOGIN_FAILS", "1") },
			func(e *testEnv) { e.claude.set("LOGIN_FAILS", "") }},
		{"claude exits zero and signs nobody in",
			func(e *testEnv) { e.claude.set("LOGIN_NOOP", "1") },
			func(e *testEnv) { e.claude.set("LOGIN_NOOP", "") }},
		{"the email belongs to another account",
			func(e *testEnv) {
				e.signIn("other", "a@example.com")
				e.claude.signInAs("a@example.com")
			},
			func(e *testEnv) { e.claude.signInAs("b@example.com") }},
		{"there is no shared claude home",
			func(e *testEnv) {
				if err := os.Remove(filepath.Join(e.home, ".claude")); err != nil {
					t.Fatal(err)
				}
			},
			func(e *testEnv) {
				if err := os.Mkdir(filepath.Join(e.home, ".claude"), 0o755); err != nil {
					t.Fatal(err)
				}
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.claude.signInAs("b@example.com")
			c.setup(e)
			e.reset()

			if code := e.run("login", "fresh"); code != 1 {
				t.Fatalf("exit %d, want 1: %s", code, e.errOut.String())
			}

			acc := e.account("fresh")
			if !acc.Meta.Hidden || acc.Meta.Email != "" {
				t.Errorf("meta = %+v, want a hidden account with no identity", acc.Meta)
			}
			for _, label := range labelsInList(t, e) {
				if label == "fresh" {
					t.Error("the account is in the list")
				}
			}

			// a retry reuses the same account and its folder
			c.fix(e)
			e.reset()
			if code := e.run("login", "fresh"); code != 0 {
				t.Fatalf("retry: exit %d: %s", code, e.errOut.String())
			}
			again := e.account("fresh")
			if again.Name != acc.Name || again.Meta.Hidden || again.Meta.Email != "b@example.com" {
				t.Errorf("after the retry: %+v, was %+v", again, acc)
			}
		})
	}
}

// when a login may be left in the folder, the account stays in the list: a
// hidden account cannot be signed out.
func TestAFirstLoginThatMayHaveLeftALoginBehindStaysInTheList(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *testEnv)
	}{
		{"the status check breaks", func(e *testEnv) { e.claude.set("STATUS_BROKEN", "1") }},
		{"an api key helper overrides the login", func(e *testEnv) { e.claude.set("API_KEY_HELPER", "1") }},
		{"the sign-out after a refusal fails", func(e *testEnv) {
			e.signIn("other", "a@example.com")
			e.claude.signInAs("a@example.com")
			e.claude.set("LOGOUT_FAILS", "1")
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.claude.signInAs("b@example.com")
			c.setup(e)
			e.reset()

			if code := e.run("login", "fresh"); code != 1 {
				t.Fatalf("exit %d, want 1: %s", code, e.errOut.String())
			}

			if e.account("fresh").Meta.Hidden {
				t.Error("the account was hidden, but a login may be left in its folder")
			}
		})
	}
}

func TestAnAccountThatFailsToSignInAgainIsNotHidden(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.claude.signInAs("b@example.com")

	if code := e.run("login", "work"); code != 1 || !strings.Contains(e.errOut.String(), "the sign-in was b@example.com") {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if e.account("work").Meta.Hidden {
		t.Error("an account that existed before this login is not hidden by its failure")
	}
}
