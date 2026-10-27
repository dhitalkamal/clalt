package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/pick"
	"github.com/dhitalkamal/clalt/internal/terminal"
)

// readyToResume signs work in, makes a conversation that ran in a folder that
// exists, and returns the folder.
func readyToResume(t *testing.T) (*testEnv, *fakeDeps, string) {
	t.Helper()
	e := newEnv(t)
	f := e.useFakeDeps()
	e.signIn("work", "a@example.com")
	cwd := t.TempDir()
	e.seedConversation(convID(7), cwd, "continue me", "2026-10-22T11:00:00Z")
	return e, f, cwd
}

func realPath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestResumeRunsTheConversationAsTheAccountFromItsFolder(t *testing.T) {
	e, _, cwd := readyToResume(t)
	before, _ := os.Getwd()

	code := e.run("resume", "aaaaaaaa", "--account", "work")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	argv, env, ran := e.claude.session()
	if want := []string{"--resume", convID(7)}; !reflect.DeepEqual(argv, want) {
		t.Errorf("claude got %v, want %v", argv, want)
	}
	if env["CLAUDE_CONFIG_DIR"] != e.folder("work") {
		t.Errorf("CLAUDE_CONFIG_DIR = %q", env["CLAUDE_CONFIG_DIR"])
	}
	if realPath(t, ran) != realPath(t, cwd) {
		t.Errorf("claude ran in %q, want %q", ran, cwd)
	}
	if realPath(t, env["PWD"]) != realPath(t, cwd) {
		t.Errorf("PWD = %q, want %q", env["PWD"], cwd)
	}
	if after, _ := os.Getwd(); after != before {
		t.Errorf("the working folder stayed %q, was %q", after, before)
	}
	if got := os.Getenv("PWD"); got != "" && realPath(t, got) == realPath(t, cwd) && realPath(t, before) != realPath(t, cwd) {
		t.Errorf("PWD stayed %q", got)
	}
	if !strings.Contains(e.errOut.String(), "work") || !strings.Contains(e.errOut.String(), "aaaaaaaa") {
		t.Errorf("no note of what is resumed: %q", e.errOut.String())
	}
}

func TestResumeAcceptsAccountAsAFlagValueWithEquals(t *testing.T) {
	e, _, _ := readyToResume(t)

	if code := e.run("resume", convID(7), "--account=work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
}

func TestResumeExitsWithClaudesCode(t *testing.T) {
	e, _, _ := readyToResume(t)
	e.claude.set("EXIT", "7")

	if code := e.run("resume", "aaaaaaaa", "--account", "work"); code != 7 {
		t.Errorf("exit %d, want claude's 7", code)
	}
}

func TestResumeUsesTheAccountThatLastRanTheConversation(t *testing.T) {
	e, f, _ := readyToResume(t)
	f.lastSeen[convID(7)] = e.account("work").Name

	if code := e.run("resume", "aaaaaaaa"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if _, env, _ := e.claude.session(); env["CLAUDE_CONFIG_DIR"] != e.folder("work") {
		t.Errorf("ran in %q", env["CLAUDE_CONFIG_DIR"])
	}
}

func TestResumeFailsWithoutAnyAccount(t *testing.T) {
	e := newEnv(t)
	e.useFakeDeps()
	e.seedConversation(convID(7), t.TempDir(), "continue me", "2026-10-22T11:00:00Z")

	code := e.run("resume", "aaaaaaaa")

	if code == 0 || e.claude.sessionStarted() || !strings.Contains(e.errOut.String(), "clalt login") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}

func TestResumeIgnoresALastAccountThatWasRemoved(t *testing.T) {
	e, f, _ := readyToResume(t)
	f.lastSeen[convID(7)] = e.account("work").Name
	if code := e.run("remove", "work", "--force"); code != 0 {
		t.Fatalf("remove: %s", e.errOut.String())
	}
	e.reset()

	if code := e.run("resume", "aaaaaaaa"); code != 1 || e.claude.sessionStarted() {
		t.Errorf("exit %d, started %v: %s", code, e.claude.sessionStarted(), e.errOut.String())
	}
}

func TestResumeRefusesAnUnknownAccount(t *testing.T) {
	e, _, _ := readyToResume(t)

	code := e.run("resume", "aaaaaaaa", "--account", "nobody")

	if code != 1 || !strings.Contains(e.errOut.String(), "nobody") || e.claude.sessionStarted() {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}

func TestResumeSaysWhenTheFolderIsGoneAndRunsNothing(t *testing.T) {
	e, _, cwd := readyToResume(t)
	if err := os.RemoveAll(cwd); err != nil {
		t.Fatal(err)
	}

	code := e.run("resume", "aaaaaaaa", "--account", "work")

	out := e.errOut.String()
	if code != 1 || !strings.Contains(out, "no longer exists") || !strings.Contains(out, cwd) ||
		!strings.Contains(out, "clalt run work -- --resume "+convID(7)) {
		t.Errorf("exit %d: %s", code, out)
	}
	if e.claude.sessionStarted() {
		t.Error("claude was started from somewhere else")
	}
}

func TestResumeSaysWhenTheFolderIsAFile(t *testing.T) {
	e, _, cwd := readyToResume(t)
	os.RemoveAll(cwd)
	if err := os.WriteFile(cwd, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := e.run("resume", "aaaaaaaa", "--account", "work"); code != 1 || e.claude.sessionStarted() {
		t.Errorf("exit %d, started %v", code, e.claude.sessionStarted())
	}
}

func TestResumeRefusesAConversationThatRunsRightNow(t *testing.T) {
	e, f, _ := readyToResume(t)
	f.running = []terminal.Session{{Handle: "w3k9", ClaudeSessionID: convID(7)}}

	code := e.run("resume", "aaaaaaaa", "--account", "work")

	out := e.errOut.String()
	if code != 1 || !strings.Contains(out, "clalt jump w3k9") || !strings.Contains(out, "clalt run work -- --resume") {
		t.Errorf("exit %d: %s", code, out)
	}
	if e.claude.sessionStarted() {
		t.Error("claude was started on a conversation that is open")
	}
}

func TestResumeNamesAnIdThatMatchesNothing(t *testing.T) {
	e, _, _ := readyToResume(t)

	code := e.run("resume", "cccccccc", "--account", "work")

	if code != 1 || !strings.Contains(e.errOut.String(), "cccccccc") || e.claude.sessionStarted() {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
	if strings.Contains(e.out.String(), "continue me") {
		t.Error("an id that matches nothing was taken for a filter")
	}
}

func TestResumeListsTheMatchesOfAnAmbiguousPrefix(t *testing.T) {
	e, _, cwd := readyToResume(t)
	e.seedConversation(convID(8), cwd, "the other one", "2026-10-21T11:00:00Z")

	code := e.run("resume", "aaaaaaaa", "--account", "work")

	out := e.errOut.String()
	if code != 1 || !strings.Contains(out, "continue me") || !strings.Contains(out, "the other one") || e.claude.sessionStarted() {
		t.Errorf("exit %d: %s", code, out)
	}
}

func TestResumeTakesAShortWordForAFilterNotAnId(t *testing.T) {
	e, _, _ := readyToResume(t)

	// "cafe" is hex, but shorter than an id is shown, so it is a filter
	if code := e.run("resume", "cafe"); code != 0 || e.claude.sessionStarted() {
		t.Errorf("exit %d, started %v", code, e.claude.sessionStarted())
	}
}

func TestResumeKeepsRunsChecks(t *testing.T) {
	e, _, _ := readyToResume(t)
	e.claude.signOut(e.folder("work"))

	code := e.run("resume", "aaaaaaaa", "--account", "work")

	if code != 1 || !strings.Contains(e.errOut.String(), "clalt login work") || e.claude.sessionStarted() {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}

// a relative folder in a transcript would be read against wherever resume runs
func TestResumeRefusesARelativeFolder(t *testing.T) {
	e := newEnv(t)
	e.useFakeDeps()
	e.signIn("work", "a@example.com")
	here := t.TempDir()
	if err := os.Mkdir(filepath.Join(here, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(here)
	e.seedConversation(convID(7), "sub", "relative", "2026-10-22T11:00:00Z")

	code := e.run("resume", "aaaaaaaa", "--account", "work")

	if code != 1 || e.claude.sessionStarted() || !strings.Contains(e.errOut.String(), "not an absolute path") {
		t.Errorf("exit %d, started %v: %s", code, e.claude.sessionStarted(), e.errOut.String())
	}
}

// with nothing known about the conversation, the picker chooses the allowed
// account with the most room, as `clalt run` does.
func TestResumePicksAnAccountWhenNoneIsKnown(t *testing.T) {
	e, _, _ := readyToResume(t)
	e.signIn("home", "b@example.com")
	withLimits(t, e, map[string]pick.Limits{"work": seenLimits(80, 10), "home": seenLimits(10, 10)})

	if code := e.run("resume", "aaaaaaaa"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if _, env, _ := e.claude.session(); env["CLAUDE_CONFIG_DIR"] != e.folder("home") {
		t.Errorf("ran in %q: %s", env["CLAUDE_CONFIG_DIR"], e.errOut.String())
	}
}

// an attributed account whose folder rules now exclude the conversation's
// folder is not used without --account: the picker chooses another.
func TestResumeSkipsAnAttributedAccountTheRulesForbid(t *testing.T) {
	e, f, _ := readyToResume(t)
	e.signIn("home", "b@example.com")
	f.lastSeen[convID(7)] = e.account("work").Name
	e.setRules("work", account.Rules{Folders: []string{subfolder(t, "elsewhere")}})

	if code := e.run("resume", "aaaaaaaa"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if _, env, _ := e.claude.session(); env["CLAUDE_CONFIG_DIR"] != e.folder("home") {
		t.Errorf("ran in %q", env["CLAUDE_CONFIG_DIR"])
	}
	if !strings.Contains(e.errOut.String(), "rules") {
		t.Errorf("no word on why: %s", e.errOut.String())
	}
}

// an explicit --account keeps working whatever the rules say.
func TestResumeAccountFlagIgnoresFolderRules(t *testing.T) {
	e, _, _ := readyToResume(t)
	e.setRules("work", account.Rules{Folders: []string{subfolder(t, "elsewhere")}})

	if code := e.run("resume", "aaaaaaaa", "--account", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
}

// the picked line comes only once the folder and open checks have passed.
func TestResumeSaysNothingOfAPickWhenTheFolderIsGone(t *testing.T) {
	e, _, cwd := readyToResume(t)
	if err := os.RemoveAll(cwd); err != nil {
		t.Fatal(err)
	}

	code := e.run("resume", "aaaaaaaa")

	if code != 1 || strings.Contains(e.errOut.String(), "picked") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}
