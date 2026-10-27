package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// warningLines are the lines of stderr that are warnings. what the sync did is
// on stderr too, and is not a warning.
func warningLines(stderr string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
		if strings.HasPrefix(line, "warning: ") {
			lines = append(lines, line)
		}
	}
	return lines
}

// wantOneHistoryWarning checks that stderr holds a single warning, and that it
// says what run will do and where to read more.
func wantOneHistoryWarning(t *testing.T, e *testEnv, label string) {
	t.Helper()
	lines := warningLines(e.errOut.String())
	if len(lines) != 1 {
		t.Fatalf("want one warning line on stderr, got %q", e.errOut.String())
	}
	msg := lines[0]
	shared := filepath.Join(e.home, ".claude", "projects")
	for _, want := range []string{label, shared, "clalt run " + label, "will refuse", "clalt doctor"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the warning is missing %q: %s", want, msg)
		}
	}
}

// run will refuse an account whose projects does not reach the shared history.
// login does not refuse for it, the sign-in goes on, but right after the sync it
// says so once instead of leaving the first run to find out.
func TestLoginWarnsOnceWhenTheSharedHistoryIsNotReachable(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	unmountHistory(t, e)
	e.claude.signInAs("a@example.com")

	code := e.run("login", "work")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	wantOneHistoryWarning(t, e, "work")
	if !strings.Contains(e.out.String(), "work is signed in as a@example.com") {
		t.Errorf("the sign-in was not reported:\n%s", e.out.String())
	}
	if got := e.account("work").Meta.Email; got != "a@example.com" {
		t.Errorf("recorded email = %q", got)
	}
}

func TestLoginWarnsAgainForAnAccountThatIsSignedInAlready(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	unmountHistory(t, e)

	e.reset()
	code := e.run("login", "work")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	wantOneHistoryWarning(t, e, "work")
}

func TestLoginWarnsWhenTheAccountHasAProjectsFolderOfItsOwn(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	acc, err := e.app.createAccount("work")
	if err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, filepath.Join(accountDir(t, acc.Name), "projects", "-mine"))
	e.claude.signInAs("a@example.com")

	code := e.run("login", "work")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	wantOneHistoryWarning(t, e, "work")
}

func TestLoginSaysNothingExtraWhenTheHistoryIsShared(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.claude.signInAs("a@example.com")

	code := e.run("login", "work")

	if code != 0 || len(warningLines(e.errOut.String())) != 0 {
		t.Errorf("exit %d, stderr %q", code, e.errOut.String())
	}
}

// the warning is about the folder, which the sync has just looked at, and not
// about how the sign-in goes. so it is printed before the sign-in starts, with
// the rest of what the sync had to say, and a sign-in that fails still has it
func TestTheHistoryWarningComesBeforeTheSignInStarts(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	unmountHistory(t, e)
	e.app.Err = e.out // one stream, to see the order
	e.claude.signInAs("a@example.com")

	if code := e.run("login", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.out.String())
	}

	out := e.out.String()
	warning, tip, done := strings.Index(out, "warning: "), strings.Index(out, "tip: "), strings.Index(out, "is signed in as")
	if warning < 0 || tip < 0 || done < 0 || warning > tip || tip > done {
		t.Errorf("want the warning, then the tip that comes before claude starts, then the result:\n%s", out)
	}
}

func TestAFailedLoginHasTheHistoryWarningAndItsOwnError(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	unmountHistory(t, e)
	e.claude.signInAs("a@example.com")
	e.claude.set("LOGIN_FAILS", "1")

	code := e.run("login", "work")

	msg := e.errOut.String()
	if code != 1 || strings.Count(msg, "will refuse") != 1 || !strings.Contains(msg, "did not finish") {
		t.Errorf("exit %d, want 1 with the warning once and the error:\n%s", code, msg)
	}
}

// a sync that cannot run has said why, and nothing was looked at
func TestLoginWithoutASharedHomeSaysOnlyThat(t *testing.T) {
	e := newEnv(t)
	if err := os.RemoveAll(filepath.Join(e.home, ".claude")); err != nil {
		t.Fatal(err)
	}
	e.claude.signInAs("a@example.com")

	code := e.run("login", "work")

	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, "does not exist") || strings.Contains(msg, "will refuse") {
		t.Errorf("exit %d, want 1 with the missing home and no history warning:\n%s", code, msg)
	}
}
