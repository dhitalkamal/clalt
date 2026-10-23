package cli

import (
	"strings"
	"testing"
	"time"
)

// real claude 2.1.303 has no SIGINT handler, so cancelling an auth call ends it
// at once, possibly in the middle of a token refresh, which can sign the folder
// out. the timeout is a last resort for a claude that never answers, not a way
// to be quick.
func TestAuthCallsAreNotCutShort(t *testing.T) {
	if authTimeout < 2*time.Minute {
		t.Errorf("authTimeout = %v, want at least 2 minutes", authTimeout)
	}
}

func TestLogoutUsesTheSameTimeoutAsStatus(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.set("LOGOUT_HANGS", "1")
	old := authTimeout
	authTimeout = 300 * time.Millisecond
	t.Cleanup(func() { authTimeout = old })

	started := time.Now()
	code := e.run("logout", "work")

	if time.Since(started) > 5*time.Second {
		t.Error("logout waited for a claude that never answered")
	}
	if code != 1 || !strings.Contains(e.errOut.String(), "may be signed out already") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
	if m := e.account("work").Meta; m.Email == "" || m.SignedInAt.IsZero() {
		t.Errorf("a logout that did not finish changed the record: %+v", m)
	}
}

func TestSameEmailNeverMatchesAnEmptyOne(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"a@example.com", "a@example.com", true},
		{"a@example.com", "A@Example.COM", true},
		{"a@example.com", "b@example.com", false},
		{"", "a@example.com", false},
		{"a@example.com", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		if got := sameEmail(c.a, c.b); got != c.want {
			t.Errorf("sameEmail(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// claude can sign a folder in and still not say as whom. that is not a match for
// the recorded email, and it is not the same as signed in as somebody else.
func TestAnEmailClaudeDoesNotReportIsNotAMatch(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.plantLogin(e.folder("work"), "")

	if code := e.run("run", "work"); code != 1 || !strings.Contains(e.errOut.String(), "without an email") {
		t.Errorf("run: exit %d: %s", code, e.errOut.String())
	}
	if e.claude.sessionStarted() {
		t.Error("a session was started")
	}
	row := listJSON(t, e)[0]
	if row["signedIn"] != false || !strings.Contains(row["reason"].(string), "without an email") {
		t.Errorf("list row = %v", row)
	}
	_, rep := doctor(t, e)
	rep.want(t, "work", "signedIn", "warn", "without an email")
}
