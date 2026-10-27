package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

func whoamiJSON(t *testing.T, e *testEnv) map[string]any {
	t.Helper()
	e.reset()
	if code := e.run("whoami", "--json"); code != 0 {
		t.Fatalf("whoami --json exited %d: %s", code, e.errOut.String())
	}
	var got map[string]any
	if err := json.Unmarshal(e.out.Bytes(), &got); err != nil {
		t.Fatalf("not json: %v\n%s", err, e.out.String())
	}
	return got
}

func TestWhoamiInsideAnAccountFolderShowsTheAccountAndItsLiveState(t *testing.T) {
	e := newEnv(t)
	e.at(theClock)
	e.signIn("work", "w@example.com")
	t.Setenv("CLAUDE_CONFIG_DIR", e.folder("work"))

	code := e.run("whoami")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	out := e.out.String()
	for _, want := range []string{"work", "w@example.com", "max", "Example Org", "signed in", e.folder("work"), dateOf(loginDue(theClock))} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
	got := whoamiJSON(t, e)
	if got["kind"] != "account" || got["label"] != "work" || got["signedIn"] != true || got["folder"] != e.folder("work") || got["email"] != "w@example.com" {
		t.Errorf("json = %v", got)
	}
}

func TestWhoamiSaysWhenTheAccountIsSignedOut(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.signOut(e.folder("work"))
	t.Setenv("CLAUDE_CONFIG_DIR", e.folder("work"))

	e.run("whoami")

	if out := e.out.String(); !strings.Contains(out, "not signed in") || !strings.Contains(out, "clalt login work") {
		t.Errorf("output:\n%s", out)
	}
	if got := whoamiJSON(t, e); got["signedIn"] != false || got["reason"] != "not signed in" {
		t.Errorf("json = %v", got)
	}
}

func TestWhoamiFindsTheAccountWhicheverWayThePathIsSpelled(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	real, err := filepath.EvalSymlinks(e.folder("work"))
	if err != nil {
		t.Fatal(err)
	}
	// not cleaned, on purpose: Join would drop the dot dot
	sep := string(filepath.Separator)
	t.Setenv("CLAUDE_CONFIG_DIR", real+sep+"sub"+sep+"..")

	if got := whoamiJSON(t, e); got["kind"] != "account" || got["label"] != "work" {
		t.Errorf("json = %v", got)
	}
}

func TestWhoamiInAnOldProfileSessionSaysSo(t *testing.T) {
	e := newEnv(t)
	old := filepath.Join(config.ProfilesDir(), "work")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", old)

	e.run("whoami")

	out := e.out.String()
	if !strings.Contains(out, "old v0.1.0 profile") || !strings.Contains(out, "clalt migrate") {
		t.Errorf("output:\n%s", out)
	}
	if got := whoamiJSON(t, e); got["kind"] != "oldProfile" || got["folder"] != old {
		t.Errorf("json = %v", got)
	}
}

func TestWhoamiOutsideAnyClaltAccount(t *testing.T) {
	for name, dir := range map[string]string{"unset": "", "elsewhere": "/somewhere/else"} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			t.Setenv("CLAUDE_CONFIG_DIR", dir)

			e.run("whoami")

			if out := e.out.String(); !strings.Contains(out, "not in a clalt account") || !strings.Contains(out, "normal login") {
				t.Errorf("output:\n%s", out)
			}
			if got := whoamiJSON(t, e); got["kind"] != "none" {
				t.Errorf("json = %v", got)
			}
		})
	}
}

func TestWhoamiInAFolderClaltDoesNotKnow(t *testing.T) {
	e := newEnv(t)
	stray := filepath.Join(config.AccountsDir(), "stray")
	t.Setenv("CLAUDE_CONFIG_DIR", stray)

	e.run("whoami")

	if out := e.out.String(); !strings.Contains(out, "stray") || !strings.Contains(out, "does not know") {
		t.Errorf("output:\n%s", out)
	}
	if got := whoamiJSON(t, e); got["kind"] != "unknownAccount" {
		t.Errorf("json = %v", got)
	}
}

func TestWhoamiInTheFolderOfARemovedAccount(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	folder := e.folder("work")
	e.run("remove", "work", "--force")
	t.Setenv("CLAUDE_CONFIG_DIR", folder)

	e.reset()
	e.run("whoami")

	if out := e.out.String(); !strings.Contains(out, "removed") || !strings.Contains(out, "clalt login work") {
		t.Errorf("output:\n%s", out)
	}
}

func TestWhoamiTakesNoArguments(t *testing.T) {
	e := newEnv(t)
	if code := e.run("whoami", "work"); code != 2 || !strings.Contains(e.errOut.String(), "usage: clalt whoami") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}
