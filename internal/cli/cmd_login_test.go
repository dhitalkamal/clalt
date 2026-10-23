package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func dueDate(signedIn time.Time) string {
	return signedIn.Add(30 * 24 * time.Hour).Local().Format("2006-01-02")
}

func TestLoginCreatesTheAccountAndRecordsWhoSignedIn(t *testing.T) {
	e := newEnv(t)
	e.at(theClock)
	e.claude.signInAs("a@example.com")

	code := e.run("login", "work")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	acc := e.account("work")
	m := acc.Meta
	if m.Email != "a@example.com" || m.Plan != "max" || m.Org != "Example Org" || m.Hidden || !m.SignedInAt.Equal(theClock) {
		t.Errorf("meta = %+v", m)
	}
	if fi, err := os.Stat(e.folder("work")); err != nil || !fi.IsDir() {
		t.Errorf("the account folder is missing: %v", err)
	}
	out := e.out.String()
	for _, want := range []string{
		"work is signed in as a@example.com (max, Example Org)",
		"sign in again by " + dueDate(theClock),
		"Login successful", // claude's own output reaches the terminal
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
	calls := e.claude.calls("login")
	if len(calls) != 1 || calls[0] != e.folder("work")+"|auth login" {
		t.Errorf("claude login calls = %q, want one bare `auth login` in the account folder", calls)
	}
}

func TestLoginTellsAboutPrivateWindowsBeforeClaudeRuns(t *testing.T) {
	e := newEnv(t)
	e.claude.signInAs("a@example.com")

	e.run("login", "work")

	out := e.out.String()
	tip, claude := strings.Index(out, "private window"), strings.Index(out, "Login successful")
	if tip < 0 || claude < 0 || tip > claude {
		t.Errorf("the private window tip should come first:\n%s", out)
	}
}

func TestLoginNamesTheStrippedVariablesAndNeverTheirValues(t *testing.T) {
	e := newEnv(t)
	e.claude.signInAs("a@example.com")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-secret")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "tok-test-secret")

	e.run("login", "work")

	out := e.out.String() + e.errOut.String()
	for _, want := range []string{"ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN"} {
		if !strings.Contains(out, want) {
			t.Errorf("%s is not named:\n%s", want, out)
		}
	}
	for _, secret := range []string{"sk-test-secret", "tok-test-secret"} {
		if strings.Contains(out, secret) {
			t.Errorf("a value leaked into the output:\n%s", out)
		}
	}
}

func TestLoginSaysNothingAboutVariablesWhenNoneAreSet(t *testing.T) {
	e := newEnv(t)
	e.claude.signInAs("a@example.com")

	e.run("login", "work")

	if msg := e.out.String() + e.errOut.String(); strings.Contains(msg, "not passing") {
		t.Errorf("no variable was stripped, yet:\n%s", msg)
	}
}

func TestLoginAgainPassesTheRecordedEmailAndRefreshesTheDate(t *testing.T) {
	e := newEnv(t)
	e.at(theClock)
	e.signIn("work", "a@example.com")
	later := theClock.Add(10 * 24 * time.Hour)
	e.at(later)

	code := e.run("login", "work")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	calls := e.claude.calls("login")
	if len(calls) != 2 || !strings.HasSuffix(calls[1], "|auth login --email a@example.com") || strings.Contains(calls[0], "--email") {
		t.Errorf("login calls = %q: only the second should carry the email", calls)
	}
	if got := e.account("work").Meta.SignedInAt; !got.Equal(later) {
		t.Errorf("signedInAt = %v, want %v", got, later)
	}
	if !strings.Contains(e.out.String(), "sign in again by "+dueDate(later)) {
		t.Errorf("the new due date is not shown:\n%s", e.out.String())
	}
}

func TestLoginFindsTheAccountWhateverTheCase(t *testing.T) {
	e := newEnv(t)
	e.signIn("Work", "a@example.com")

	e.claude.signInAs("a@example.com")
	if code := e.run("login", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	list, _ := e.app.Store.List()
	if len(list) != 1 || list[0].Label() != "Work" {
		t.Errorf("accounts = %+v, want the one called Work", list)
	}
}

func TestLoginBringsAHiddenAccountBackAsAFreshIdentity(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	if err := e.app.Store.Hide(e.account("work").Name); err != nil {
		t.Fatal(err)
	}
	e.claude.signInAs("b@example.com")

	code := e.run("login", "work")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	m := e.account("work").Meta
	if m.Hidden || m.Email != "b@example.com" {
		t.Errorf("meta = %+v", m)
	}
	// remove cleared the email, so nothing is checked against the old one
	calls := e.claude.calls("login")
	if last := calls[len(calls)-1]; strings.Contains(last, "--email") {
		t.Errorf("a hidden account has no email to prefill: %s", last)
	}
}

func TestLoginKeepsAHiddenAccountHiddenUntilItSucceeds(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	if code := e.run("remove", "work", "--force"); code != 0 {
		t.Fatalf("remove exited %d: %s", code, e.errOut.String())
	}
	e.claude.set("LOGIN_FAILS", "1")

	if code := e.run("login", "work"); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}

	if !e.account("work").Meta.Hidden {
		t.Error("a failed login brought the account back")
	}
}

func TestLoginRejectsALabelThatCannotBeAFolderName(t *testing.T) {
	e := newEnv(t)

	code := e.run("login", "../x")

	if code != 1 || len(e.claude.calls("login")) != 0 {
		t.Errorf("exit %d, login calls %v: nothing should run for a bad name", code, e.claude.calls("login"))
	}
	if list, _ := e.app.Store.List(); len(list) != 0 {
		t.Errorf("accounts = %+v", list)
	}
}

func TestLoginMistakesAreUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"login"}, {"login", "a", "b"}, {"login", "--nope"}} {
		e := newEnv(t)
		code := e.run(args...)
		if code != 2 || !strings.Contains(e.errOut.String(), "usage: clalt login <name>") {
			t.Errorf("%v: exit %d, stderr %q", args, code, e.errOut.String())
		}
	}
}

func TestLoginGivesClaudeTheTerminal(t *testing.T) {
	e := newEnv(t)
	e.claude.signInAs("a@example.com")
	e.claude.set("READS_STDIN", "1")
	e.app.In = strings.NewReader("pasted-code\n")

	if code := e.run("login", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := e.claude.stdinSeen(); got != "pasted-code" {
		t.Errorf("claude read %q from stdin", got)
	}
}

func TestLoginLinksTheSharedHomeAndMirrorsMCPServers(t *testing.T) {
	e := newEnv(t)
	shared := e.seedShared()
	e.claude.signInAs("a@example.com")

	if code := e.run("login", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	folder := e.folder("work")
	target, err := os.Readlink(filepath.Join(folder, "projects"))
	if err != nil || target != filepath.Join(shared, "projects") {
		t.Errorf("projects links to %q (%v), want the shared projects", target, err)
	}
	raw, err := os.ReadFile(filepath.Join(folder, ".claude.json"))
	if err != nil {
		t.Fatalf("no .claude.json: %v", err)
	}
	var cfg struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil || cfg.MCPServers["github"] == nil {
		t.Errorf("the shared mcp server did not arrive: %s", raw)
	}
	if msg := e.errOut.String(); !strings.Contains(msg, "2 items linked") || !strings.Contains(msg, "1 mcp server added") {
		t.Errorf("what was synced is not reported:\n%s", msg)
	}
}
