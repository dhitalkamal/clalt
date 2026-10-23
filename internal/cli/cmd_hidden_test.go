package cli

import (
	"strings"
	"testing"
)

// account.Store.List returns removed accounts too. every command that shows or
// walks the accounts decides what to do with them on purpose, and this is the
// one place that says so: they are left out everywhere, except where a label is
// only looked up.
func removedAccountEnv(t *testing.T) *testEnv {
	t.Helper()
	e := newEnv(t)
	e.signIn("kept", "k@example.com")
	e.signIn("gone", "g@example.com")
	if code := e.run("remove", "gone", "--force"); code != 0 {
		t.Fatalf("remove exited %d: %s", code, e.errOut.String())
	}
	e.reset()
	return e
}

func TestRemovedAccountsAreLeftOutOfTheViews(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		e := removedAccountEnv(t)
		rows := listJSON(t, e)
		if len(rows) != 1 || rows[0]["label"] != "kept" {
			t.Errorf("rows = %v", rows)
		}
	})
	t.Run("doctor", func(t *testing.T) {
		e := removedAccountEnv(t)
		_, rep := doctor(t, e)
		for _, c := range rep.Checks {
			if c.Scope == "gone" {
				t.Errorf("doctor checked a removed account: %+v", c)
			}
		}
		rep.want(t, "kept", "signedIn", "ok", "")
	})
	t.Run("usage", func(t *testing.T) {
		e := removedAccountEnv(t)
		rows := usageRows(t, e)
		if len(rows) != 1 || rows[0]["name"] != "kept" {
			t.Errorf("rows = %v", rows)
		}
	})
	t.Run("budget", func(t *testing.T) {
		e := removedAccountEnv(t)
		rows := budgetRows(t, e)
		if len(rows) != 1 || rows[0]["name"] != "kept" {
			t.Errorf("rows = %v", rows)
		}
	})
	t.Run("the duplicate email check of login", func(t *testing.T) {
		e := removedAccountEnv(t)
		// remove forgot the email, so a hand-written state file is the only way
		// a removed account still has one
		plantState(t, `{"active":"","accounts":{"gone":{"email":"g@example.com","addedAt":"2026-01-01T00:00:00Z","hidden":true}}}`)
		e.claude.signInAs("G@example.com")

		if code := e.run("login", "fresh"); code != 0 {
			t.Errorf("a removed account blocked a login: exit %d: %s", code, e.errOut.String())
		}
	})
}

// a session started before its account was removed is still running, and it is
// shown by the name it had.
func TestSessionsOfARemovedAccountKeepTheirLabel(t *testing.T) {
	e := removedAccountEnv(t)
	if code := e.run("rename", "gone", "ex-gone"); code != 0 { // so that the label is not the id
		t.Fatalf("rename exited %d: %s", code, e.errOut.String())
	}
	liveSession(t, "gone", "/code/x")
	e.reset()

	e.run("sessions")

	if out := e.out.String(); !strings.Contains(out, "ex-gone") || !strings.Contains(out, "/code/x") {
		t.Errorf("output:\n%s", out)
	}
}

// the commands that look an account up by label can still find a removed one,
// and each of them says what it does with it.
func TestLookingUpARemovedAccountByLabel(t *testing.T) {
	cases := []struct {
		args []string
		code int
		want string
	}{
		{[]string{"run", "gone"}, 1, "was removed"},
		{[]string{"logout", "gone"}, 0, "stays removed"},
		{[]string{"remove", "gone", "--force"}, 1, "already removed"},
		{[]string{"usage", "gone"}, 1, "was removed"},
		{[]string{"budget", "gone", "--daily", "1"}, 1, "was removed"},
		{[]string{"rename", "gone", "old-gone"}, 0, "renamed"},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			e := removedAccountEnv(t)

			code := e.run(c.args...)

			if code != c.code || !strings.Contains(e.errOut.String()+e.out.String(), c.want) {
				t.Errorf("exit %d, stdout %q, stderr %q, want %d and %q", code, e.out.String(), e.errOut.String(), c.code, c.want)
			}
		})
	}
}
