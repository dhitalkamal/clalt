package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// the fix that is printed names the label, which is what login takes
func TestRefusalAdviceUsesTheLabelOfARenamedAccount(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	id := e.account("work").Name
	if err := e.app.Store.Relabel(id, "office"); err != nil {
		t.Fatal(err)
	}
	e.claude.signOut(e.folder("office"))
	e.reset()

	code := e.run("run", "office")

	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, "clalt login office") || strings.Contains(msg, "clalt login "+id) {
		t.Errorf("exit %d: %s", code, msg)
	}
}

// the unreachable refusal names the account link first and the shared folder second
func TestUnreachableRefusalNamesBothPathsInOrder(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	unmountHistory(t, e)
	e.reset()

	code := e.run("run", "work")

	link := filepath.Join(e.folder("work"), "projects")
	shared := filepath.Join(e.home, ".claude", "projects")
	want := link + " does not lead to the shared history folder " + shared
	if code != 1 || !strings.Contains(e.errOut.String(), want) {
		t.Errorf("exit %d, want %q in:\n%s", code, want, e.errOut.String())
	}
}

// the own-folder refusal points at doctor and names the paths in order
func TestOwnFolderRefusalPointsAtDoctorAndNamesPathsInOrder(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	projects := filepath.Join(e.folder("work"), "projects")
	if err := os.Remove(projects); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projects, "-p"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.reset()

	code := e.run("run", "work")

	msg := e.errOut.String()
	shared := filepath.Join(e.home, ".claude", "projects")
	for _, want := range []string{"clalt doctor", projects + " is not linked to the shared history", "move what is in it into " + shared} {
		if !strings.Contains(msg, want) {
			t.Errorf("exit %d: missing %q in:\n%s", code, want, msg)
		}
	}
}

// an item that failed to link is not the reason when projects is only unreachable
func TestAnotherItemFailingDoesNotMakeProjectsTheCulprit(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	folder := e.folder("work")
	unmountHistory(t, e)
	if err := os.Remove(filepath.Join(folder, "settings.json")); err != nil {
		t.Fatal(err)
	}
	if !makeReadOnly(t, folder) {
		t.Skip("cannot make a folder read-only here")
	}
	e.reset()

	code := e.run("run", "work")

	msg := e.errOut.String()
	if code != 1 || strings.Contains(msg, "projects could not be linked") || !strings.Contains(msg, "does not lead to the shared history folder") {
		t.Errorf("exit %d: %s", code, msg)
	}
}

// the recorded email is compared ignoring case
func TestRunMatchesTheRecordedEmailIgnoringCase(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	e.claude.plantLogin(e.folder("work"), "A@Example.COM")
	e.reset()

	if code := e.run("run", "work"); code != 0 {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}

// a run that has nothing to say, and a claude that fails, print nothing of their own
func TestCleanRunAndFailingClaudePrintNothing(t *testing.T) {
	for _, exit := range []string{"0", "7"} {
		t.Run("exit "+exit, func(t *testing.T) {
			e := newEnv(t)
			e.signIn("work", "a@example.com")
			e.claude.set("EXIT", exit)
			e.reset()

			e.run("run", "work")

			if e.out.Len() != 0 || e.errOut.Len() != 0 {
				t.Errorf("stdout %q, stderr %q, want nothing", e.out.String(), e.errOut.String())
			}
		})
	}
}

// a provider override is not fixed by signing in again
func TestProviderOverridePointsAtDoctor(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	extendStatus(t, e, `if [ -n "$FAKE_CLAUDE_PROVIDER" ]; then printf '{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"%s","email":"a@example.com","configDirectory":"%s"}\n' "$FAKE_CLAUDE_PROVIDER" "$cfg"; exit 0; fi`)
	t.Setenv("FAKE_CLAUDE_PROVIDER", "bedrock")
	e.reset()

	code := e.run("run", "work")

	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, "bedrock") || !strings.Contains(msg, "clalt doctor") || strings.Contains(msg, "clalt login") || e.claude.sessionStarted() {
		t.Errorf("exit %d: %s", code, msg)
	}
}

// a login that claude reports without an email has expired, and signing in again is the fix
func TestLoginWithoutEmailSaysToSignInAgain(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.plantLogin(e.folder("work"), "")
	e.reset()

	code := e.run("run", "work")

	if msg := e.errOut.String(); code != 1 || !strings.Contains(msg, "clalt login work") {
		t.Errorf("exit %d: %s", code, msg)
	}
}

// extendStatus used to patch the sh fake. the Go fake already answers as the
// provider that FAKE_CLAUDE_PROVIDER names, so there is nothing to add.
func extendStatus(t *testing.T, e *testEnv, extra string) { t.Helper() }

// only what claude reports goes in the parentheses
func TestLoginShowsOnlyWhatIsKnownInParentheses(t *testing.T) {
	for _, c := range []struct{ body, want string }{
		{"a@example.com\n\nmax\n", "work is signed in as a@example.com (max)\n"},
		{"a@example.com\n\n\n", "work is signed in as a@example.com\n"},
	} {
		e := newEnv(t)
		e.signIn("work", "a@example.com")
		if err := os.WriteFile(filepath.Join(e.folder("work"), ".fake-login"), []byte(c.body), 0o600); err != nil {
			t.Fatal(err)
		}
		e.claude.set("LOGIN_NOOP", "1")
		e.reset()

		code := e.run("login", "work")

		if code != 0 || !strings.Contains(e.out.String(), c.want) {
			t.Errorf("exit %d, want %q in:\n%s%s", code, c.want, e.out.String(), e.errOut.String())
		}
	}
}
