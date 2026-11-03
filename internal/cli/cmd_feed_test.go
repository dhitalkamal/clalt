package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/defaulthome"
	"github.com/dhitalkamal/clalt/internal/usage"
)

const feedUserFile = `{
  "model": "opus",
  "hooks": {"Stop": [{"hooks": [{"type": "command", "command": "say done"}]}]},
  "statusLine": {"type": "command", "command": "echo mine", "padding": 2}
}
`

func (e *testEnv) settingsPath() string { return filepath.Join(e.home, ".claude", "settings.json") }

func (e *testEnv) settings() string {
	e.t.Helper()
	b, err := os.ReadFile(e.settingsPath())
	if err != nil {
		e.t.Fatal(err)
	}
	return string(b)
}

// feedReady is an account work whose email is also what plain claude is signed
// in as, and a settings.json of the user's.
func feedReady(t *testing.T, body string) *testEnv {
	t.Helper()
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.plainClaudeAs("W@example.com")
	if body != "" {
		if err := os.WriteFile(e.settingsPath(), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

func TestFeedInstallMergesBacksUpAndKeepsTheUsersEntries(t *testing.T) {
	skipWindows(t, feedRefusedOnWindows)
	e := feedReady(t, feedUserFile)

	code := e.run("feed", "install", "--yes")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	got := e.settings()
	for _, want := range []string{`"model": "opus"`, "say done", "hook SessionStart clalt-feed", "statusline clalt-feed", `"padding": 2`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	backups, _ := filepath.Glob(filepath.Join(config.ClaltDir(), "backups", "settings", "*.json"))
	if len(backups) != 1 {
		t.Fatalf("backups: %v", backups)
	}
	if b, _ := os.ReadFile(backups[0]); string(b) != feedUserFile {
		t.Error("the backup is not the users file")
	}
	if !strings.Contains(e.out.String(), backups[0]) {
		t.Errorf("the backup path is not shown: %s", e.out.String())
	}
}

func TestFeedInstallKeepsTheUsersStatusLineRunning(t *testing.T) {
	skipWindows(t, "the user command runs through the platform shell, and these use sh syntax")
	e := feedReady(t, feedUserFile)
	e.run("feed", "install", "--yes")
	e.reset()
	t.Setenv("CLALT_SESSION", "")

	code := e.statusline(statusPayload)

	if code != 0 || e.out.String() != "mine\n" {
		t.Errorf("exit %d out %q", code, e.out.String())
	}
	if l, ok := feedStore().Limits(e.account("work").Name); !ok || l.FiveHour == nil {
		t.Errorf("no limits for the matched account: %+v", l)
	}
}

func TestFeedInstallAsksFirstAndRefusesWhenNobodyAnswers(t *testing.T) {
	skipWindows(t, feedRefusedOnWindows)
	e := feedReady(t, feedUserFile)

	if code := e.run("feed", "install"); code == 0 {
		t.Error("installed without a yes")
	}
	e.answer("n\n")
	if code := e.run("feed", "install"); code != 0 || e.settings() != feedUserFile {
		t.Errorf("a no changed the file (exit %d)", code)
	}
	e.answer("y\n")
	if code := e.run("feed", "install"); code != 0 || !strings.Contains(e.settings(), "clalt-feed") {
		t.Errorf("a yes did not install (exit %d: %s)", code, e.errOut.String())
	}
}

func TestFeedInstallNeedsAMatchedDefaultHome(t *testing.T) {
	skipWindows(t, feedRefusedOnWindows)
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.plainClaudeAs("someone@else.com")
	os.WriteFile(e.settingsPath(), []byte(feedUserFile), 0o600)

	code := e.run("feed", "install", "--yes")

	if code == 0 || e.settings() != feedUserFile || !strings.Contains(e.errOut.String(), "someone@else.com") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}

func TestFeedInstallRefusesInvalidJson(t *testing.T) {
	skipWindows(t, feedRefusedOnWindows)
	e := feedReady(t, "")
	os.WriteFile(e.settingsPath(), []byte(`{"hooks": `), 0o600)
	e.reset()
	if code := e.run("feed", "install", "--yes"); code == 0 || !strings.Contains(e.errOut.String(), "not valid JSON") {
		t.Errorf("invalid: exit %d %s", code, e.errOut.String())
	}
	if e.settings() != `{"hooks": ` {
		t.Error("changed")
	}
}

func TestFeedInstallTwiceSaysSoAndChangesNothing(t *testing.T) {
	skipWindows(t, feedRefusedOnWindows)
	e := feedReady(t, feedUserFile)
	e.run("feed", "install", "--yes")
	after := e.settings()
	e.reset()

	code := e.run("feed", "install", "--yes")

	if code != 0 || e.settings() != after || !strings.Contains(e.out.String(), "already installed") {
		t.Errorf("exit %d out %s", code, e.out.String())
	}
	if b, _ := filepath.Glob(filepath.Join(config.ClaltDir(), "backups", "settings", "*")); len(b) != 1 {
		t.Errorf("backups: %v", b)
	}
}

func TestFeedUninstallPutsTheUsersFileBack(t *testing.T) {
	skipWindows(t, feedRefusedOnWindows)
	e := feedReady(t, feedUserFile)
	e.run("feed", "install", "--yes")
	e.reset()

	code := e.run("feed", "uninstall", "--yes")

	if code != 0 || e.settings() != feedUserFile {
		t.Errorf("exit %d, file:\n%s", code, e.settings())
	}
}

func TestFeedStatusSaysWhetherItIsInstalled(t *testing.T) {
	skipWindows(t, feedRefusedOnWindows)
	e := feedReady(t, feedUserFile)
	e.run("feed", "status")
	if !strings.Contains(e.out.String(), "not installed") {
		t.Errorf("before: %s", e.out.String())
	}
	e.run("feed", "install", "--yes")
	e.reset()

	e.run("feed", "status")

	out := e.out.String()
	if !strings.Contains(out, "installed") || strings.Contains(out, "not installed") || !strings.Contains(out, "your own statusLine") {
		t.Errorf("after: %s", out)
	}
	e.reset()
	e.run("feed", "status", "--json")
	if !strings.Contains(e.out.String(), `"installed": true`) {
		t.Errorf("json: %s", e.out.String())
	}
}

func TestFeedForgetDefaultRemovesOnlyTheDefaultLinesAndStopsTheAttribution(t *testing.T) {
	e := newEnv(t)
	e.at(loginTime)
	e.signIn("work", "w@example.com")
	e.plainClaudeAs("w@example.com")
	e.app.refreshDefaultHome()
	e.writeSwitchLog(switchToWork)
	e.writeTranscript(afterIt)
	work := e.account("work").Name
	if got := usageByName(t, e)["work"]["tokens"]; got != float64(3000) {
		t.Fatalf("not attributed: %v", got)
	}
	_ = usage.AppendSessionAccount(config.SessionAccountsPath(), usage.SessionAccount{Ts: loginTime, SessionID: "other", Account: work})

	if code := e.run("feed", "forget-default"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	lines, _ := usage.ReadSessionAccounts(config.SessionAccountsPath())
	if len(lines) != 1 || lines[0].SessionID != "other" {
		t.Errorf("ledger: %+v", lines)
	}
	if got := usageByName(t, e)["unattributed"]["tokens"]; got != float64(3000) {
		t.Errorf("unattributed %v", got)
	}
	if c, _ := defaulthome.Read(config.DefaultHomePath()); !c.Off {
		t.Error("attribution not off")
	}
	if code := e.run("feed", "remember-default"); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if c, _ := defaulthome.Read(config.DefaultHomePath()); c.Off {
		t.Error("attribution still off")
	}
}
