package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// the last lines of doctor's text say that everything passed only when nothing
// was found, and count what was found otherwise. a warning is not a pass, and
// neither is a failure
func TestDoctorSaysAllChecksPassedOnlyWhenThereIsNeitherAWarningNorAFailure(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(e *testEnv)
		exit    int
		summary string // the last line, or "" for the line that says all passed
	}{
		{"nothing found", func(e *testEnv) {
			e.seedShared()
			e.signIn("work", "w@example.com")
		}, 0, ""},
		{"a warning", func(e *testEnv) {
			e.seedShared()
			e.signIn("work", "w@example.com")
			e.run("logout", "work")
		}, 0, "0 checks failed, 1 warning"},
		{"a failure", func(e *testEnv) {
			e.claude.set("VERSION", "2.1.300")
		}, 1, "1 check failed, 0 warnings"},
		{"a failure and a warning", func(e *testEnv) {
			e.seedShared()
			e.signIn("work", "w@example.com")
			e.claude.plantLogin(e.folder("work"), "other@example.com")
			if err := os.Remove(filepath.Join(e.folder("work"), "settings.json")); err != nil {
				t.Fatal(err)
			}
		}, 1, "1 check failed, 1 warning"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			c.setup(e)
			e.reset()

			code := e.run("doctor")

			out := strings.TrimRight(e.out.String(), "\n")
			last := out[strings.LastIndex(out, "\n")+1:]
			if code != c.exit {
				t.Errorf("exit %d, want %d:\n%s", code, c.exit, out)
			}
			if c.summary == "" {
				if last != "all checks passed" || strings.Contains(out, " failed, ") {
					t.Errorf("want the line that says all checks passed to end the report:\n%s", out)
				}
				return
			}
			if last != c.summary || strings.Contains(out, "all checks passed") {
				t.Errorf("the report ends with %q, want %q and no claim that all passed:\n%s", last, c.summary, out)
			}
		})
	}
}

// the login of an account is due 30 days after the time it was recorded. an
// account with no such time (one from v0.1.0 that is signed in, or one that was
// signed out, which forgets it) has no due date to judge, and is not told that
// its login was due in the year 1
func TestDoctorHasNoLoginDueCheckForAnAccountWithoutARecordedSignIn(t *testing.T) {
	cases := map[string]func(e *testEnv){
		"an account from v0.1.0 that is signed in": func(e *testEnv) {
			plantState(t, `{"active":"work","accounts":{"work":{"email":"w@example.com","plan":"max","addedAt":"2026-08-01T10:00:00Z"}}}`)
			e.claude.plantLogin(accountDir(t, "work"), "w@example.com")
		},
		"an account that was signed out": func(e *testEnv) {
			e.signIn("work", "w@example.com")
			if code := e.run("logout", "work"); code != 0 {
				t.Fatalf("logout exited %d: %s", code, e.errOut.String())
			}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			e.seedShared()
			setup(e)
			if !e.account("work").Meta.SignedInAt.IsZero() {
				t.Fatal("the account has a recorded sign-in, which this test is not about")
			}

			code, rep := doctor(t, e)

			if c, found := rep.find("work", "loginDue"); found {
				t.Errorf("a loginDue check for an account with no recorded sign-in: %+v", c)
			}
			if strings.Contains(e.out.String(), "0001-") || code != 0 {
				t.Errorf("exit %d, a date from the year 1 in the report:\n%s", code, e.out.String())
			}
		})
	}
}
