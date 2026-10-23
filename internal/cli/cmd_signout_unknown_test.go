package cli

import (
	"strings"
	"testing"
	"time"
)

// claude can fail to sign a folder out and say so, and then the folder is still
// signed in. it can also be cut short, or leave a status that cannot be read,
// and then nobody knows. the message must not claim nothing happened.
func TestLogoutSaysWhenItCannotTellWhetherTheFolderIsSignedOut(t *testing.T) {
	cases := map[string]func(e *testEnv){
		"the check afterwards breaks": func(e *testEnv) { e.claude.set("STATUS_BROKEN", "1") },
		"the call hangs": func(e *testEnv) {
			e.claude.set("LOGOUT_HANGS", "1")
			old := authTimeout
			authTimeout = 300 * time.Millisecond
			t.Cleanup(func() { authTimeout = old })
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			e.signIn("work", "w@example.com")
			before := e.account("work").Meta
			setup(e)

			code := e.run("logout", "work")

			msg := e.errOut.String()
			for _, want := range []string{"sign-out state unknown", "may be signed out already", "clalt list", "clalt logout work"} {
				if code != 1 || !strings.Contains(msg, want) {
					t.Errorf("exit %d, missing %q: %s", code, want, msg)
				}
			}
			if strings.Contains(msg, "nothing was changed") {
				t.Errorf("the folder may well have changed: %s", msg)
			}
			if after := e.account("work").Meta; after != before {
				t.Errorf("the record changed: %+v then %+v", before, after)
			}
		})
	}
}

func TestRemoveDoesNotHideAnAccountWhoseSignOutIsUnknown(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.set("LOGOUT_HANGS", "1")
	old := authTimeout
	authTimeout = 300 * time.Millisecond
	t.Cleanup(func() { authTimeout = old })

	code := e.run("remove", "work", "--force")

	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, "the account was not removed") || !strings.Contains(msg, "may be signed out already") {
		t.Errorf("exit %d: %s", code, msg)
	}
	if m := e.account("work").Meta; m.Hidden || m.Email == "" {
		t.Errorf("the account was changed: %+v", m)
	}
}

func TestAPlainSignOutFailureStillSaysNothingWasChanged(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.set("LOGOUT_FAILS", "1")

	code := e.run("remove", "work", "--force")

	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, "nothing was changed") || strings.Contains(msg, "may be signed out already") {
		t.Errorf("exit %d: %s", code, msg)
	}
}
