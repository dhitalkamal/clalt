package cli

import (
	"strings"
	"testing"
)

func TestLoginRefusesADifferentEmailThanTheRecordedOne(t *testing.T) {
	e := newEnv(t)
	e.at(theClock)
	e.signIn("work", "a@example.com")
	e.claude.signInAs("b@example.com")

	code := e.run("login", "work")

	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	msg := e.errOut.String()
	for _, want := range []string{"set up for a@example.com", "the sign-in was b@example.com", "private browser window", "try again"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal is missing %q: %s", want, msg)
		}
	}
	if e.claude.signedIn(e.folder("work")) {
		t.Error("the folder was left signed in as the wrong account")
	}
	m := e.account("work").Meta
	if m.Email != "a@example.com" || !m.SignedInAt.IsZero() {
		t.Errorf("meta = %+v: the email stays, the sign-in is cleared", m)
	}
}

func TestLoginRefusesAnEmailThatBelongsToAnotherAccount(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.claude.signInAs("A@Example.COM")

	code := e.run("login", "home")

	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	msg := e.errOut.String()
	for _, want := range []string{"A@Example.COM already belongs to", `"work"`, "private browser window", "try again"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal is missing %q: %s", want, msg)
		}
	}
	if e.claude.signedIn(e.folder("home")) {
		t.Error("the duplicate login was kept")
	}
	if got := e.account("home").Meta.Email; got != "" {
		t.Errorf("home recorded %q", got)
	}
	if got := e.account("work").Meta.Email; got != "a@example.com" {
		t.Errorf("work changed to %q", got)
	}
}

func TestLoginSaysSoWhenTheForcedLogoutFails(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.claude.signInAs("b@example.com")
	e.claude.set("LOGOUT_FAILS", "1")

	code := e.run("login", "work")

	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, "clalt logout work") {
		t.Errorf("exit %d: %s, want a pointer to `clalt logout work`", code, msg)
	}
}

func TestLoginFailures(t *testing.T) {
	cases := []struct {
		name string
		set  [][2]string
		want string
	}{
		{"claude exits non-zero", [][2]string{{"LOGIN_FAILS", "1"}}, "claude auth login"},
		{"claude exits zero but signs nobody in", [][2]string{{"LOGIN_NOOP", "1"}}, "not signed in"},
		{"the folder falls back to an api key helper", [][2]string{{"API_KEY_HELPER", "1"}}, "api_key_helper"},
		{"the status check itself breaks", [][2]string{{"STATUS_BROKEN", "1"}}, "keychain is locked"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.claude.signInAs("a@example.com")
			for _, kv := range c.set {
				e.claude.set(kv[0], kv[1])
			}

			code := e.run("login", "work")

			if code != 1 || !strings.Contains(e.errOut.String(), c.want) {
				t.Errorf("exit %d, stderr %q, want 1 and %q", code, e.errOut.String(), c.want)
			}
			if m := e.account("work").Meta; m.Email != "" || !m.SignedInAt.IsZero() {
				t.Errorf("an identity was recorded: %+v", m)
			}
		})
	}
}

func TestLoginWithoutAnEmailFromClaudeIsAnError(t *testing.T) {
	e := newEnv(t)
	e.claude.signInAs("")

	code := e.run("login", "work")

	if code != 1 || !strings.Contains(e.errOut.String(), "email") {
		t.Errorf("exit %d, stderr %q", code, e.errOut.String())
	}
}

// remove clears the email, so a hidden account normally has none. a state file
// written by hand can still hold one, and it must not block anybody.
func TestLoginIgnoresHiddenAccountsWhenLookingForDuplicates(t *testing.T) {
	e := newEnv(t)
	plantState(t, `{"active":"","accounts":{"old":{"email":"a@example.com","addedAt":"2026-01-01T00:00:00Z","hidden":true}}}`)
	e.claude.signInAs("a@example.com")

	if code := e.run("login", "fresh"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
}

// the email that signed in comes from claude, so what the refusal prints of it
// is cleaned like everything else claude reports.
func TestTheRefusalPrintsTheEmailThatSignedInWithoutAnEscape(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.claude.signInAs(`b\u001b[2K@example.com`)

	code := e.run("login", "work")

	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, `the sign-in was b\x1b[2K@example.com`) || hasControl(msg) {
		t.Errorf("exit %d: %q", code, msg)
	}
}
