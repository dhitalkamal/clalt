package cli

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/pick"
)

// seenLimits is what the feed saw a minute before the clock the tests run at.
func seenLimits(five, seven float64) pick.Limits {
	win := func(used float64, resets time.Duration) pick.Window {
		return pick.Window{Used: used, ResetsAt: theClock.Add(resets), Known: true}
	}
	return pick.Limits{FiveHour: win(five, 2*time.Hour), SevenDay: win(seven, 72*time.Hour), ObservedAt: theClock.Add(-time.Minute)}
}

// withLimits makes the feed answer for these accounts, by label, and nothing for
// the others.
func withLimits(t *testing.T, e *testEnv, byLabel map[string]pick.Limits) {
	t.Helper()
	e.at(theClock)
	byID := map[string]pick.Limits{}
	for label, l := range byLabel {
		byID[e.account(label).Name] = l
	}
	old := feedLimits
	feedLimits = func(id string) (pick.Limits, bool) { l, ok := byID[id]; return l, ok }
	t.Cleanup(func() { feedLimits = old })
}

// ranAs is the folder of the account the last session ran as.
func ranAs(e *testEnv) string {
	e.t.Helper()
	_, env, _ := e.claude.session()
	return env["CLAUDE_CONFIG_DIR"]
}

func TestRunWithoutANamePicksTheOnlyAccount(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	if code := e.run("run"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := ranAs(e); got != e.folder("work") {
		t.Errorf("ran as %q, want %q", got, e.folder("work"))
	}
	if msg := e.errOut.String(); !strings.Contains(msg, "picked work: the only allowed account") {
		t.Errorf("stderr does not say which account was picked and why:\n%s", msg)
	}
	if strings.Contains(e.out.String(), "picked") {
		t.Errorf("stdout is claude's, and it has the pick: %q", e.out.String())
	}
}

func TestRunWithoutANamePicksTheAccountWithTheMostRoom(t *testing.T) {
	e := newEnv(t)
	e.signIn("alpha", "a@example.com")
	e.signIn("beta", "b@example.com")
	e.signIn("gamma", "g@example.com")
	withLimits(t, e, map[string]pick.Limits{"alpha": seenLimits(70, 10), "beta": seenLimits(20, 30), "gamma": seenLimits(50, 10)})

	if code := e.run("run"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := ranAs(e); got != e.folder("beta") {
		t.Errorf("ran as %q, want beta at %q", got, e.folder("beta"))
	}
	if msg := e.errOut.String(); !strings.Contains(msg, "picked beta: most room (5-hour window 20% used, 7-day window 30% used)") {
		t.Errorf("stderr:\n%s", msg)
	}
}

func TestRunWithoutANameGoesByLabelWhenNothingKnowsTheLimits(t *testing.T) {
	e := newEnv(t)
	e.signIn("beta", "b@example.com")
	e.signIn("alpha", "a@example.com")

	if code := e.run("run"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := ranAs(e); got != e.folder("alpha") {
		t.Errorf("ran as %q, want alpha", got)
	}
	if msg := e.errOut.String(); !strings.Contains(msg, "picked alpha: no recent limit data, first by label") {
		t.Errorf("stderr:\n%s", msg)
	}
}

func TestRunWithoutANameKeepsToTheFolderRules(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.signIn("home", "h@example.com")
	workDir, homeDir := subfolder(t, "work"), subfolder(t, "home")
	e.setRules("work", account.Rules{Folders: []string{workDir}})
	e.setRules("home", account.Rules{Folders: []string{homeDir}})
	// work has far more room, and is still not allowed in the home folder
	withLimits(t, e, map[string]pick.Limits{"work": seenLimits(5, 5), "home": seenLimits(80, 80)})
	t.Chdir(homeDir)

	if code := e.run("run"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := ranAs(e); got != e.folder("home") {
		t.Errorf("ran as %q, want home", got)
	}
}

func TestRunWithoutANameSaysWhyNoAccountCanRun(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	addAccount(t, e, "fresh")
	e.setRules("work", account.Rules{Folders: []string{subfolder(t, "elsewhere")}})
	here := subfolder(t, "here")
	t.Chdir(here)

	code := e.run("run")

	msg := e.errOut.String()
	if code != 1 || e.claude.sessionStarted() {
		t.Errorf("exit %d, session started %v: %s", code, e.claude.sessionStarted(), msg)
	}
	for _, want := range []string{"no account can be picked", "work: its folder rules do not include", "fresh: is not signed in", "clalt run <name>"} {
		if !strings.Contains(msg, want) {
			t.Errorf("stderr lacks %q:\n%s", want, msg)
		}
	}
}

func TestRunWithoutANameWithNoAccountsSaysHowToSignIn(t *testing.T) {
	e := newEnv(t)

	code := e.run("run")

	if code != 1 || !strings.Contains(e.errOut.String(), "clalt login <name>") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}

// the sign-in recorded in the state can be out of date: the login may be gone
// since, and claude is the one that knows
func TestRunWithoutANameSkipsAnAccountThatClaudeSaysIsSignedOut(t *testing.T) {
	e := newEnv(t)
	e.signIn("alpha", "a@example.com")
	e.signIn("beta", "b@example.com")
	withLimits(t, e, map[string]pick.Limits{"alpha": seenLimits(10, 10), "beta": seenLimits(60, 10)})
	e.claude.signOut(e.folder("alpha"))

	if code := e.run("run"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := ranAs(e); got != e.folder("beta") {
		t.Errorf("ran as %q, want beta", got)
	}
	msg := e.errOut.String()
	if !strings.Contains(msg, "skipped alpha: not signed in") || !strings.Contains(msg, "picked beta") {
		t.Errorf("stderr:\n%s", msg)
	}
}

func TestRunWithoutANameEndsWhenEveryAccountIsSignedOutAtClaude(t *testing.T) {
	e := newEnv(t)
	e.signIn("alpha", "a@example.com")
	e.claude.signOut(e.folder("alpha"))

	code := e.run("run")

	if code != 1 || e.claude.sessionStarted() || !strings.Contains(e.errOut.String(), "clalt login alpha") {
		t.Errorf("exit %d, started %v: %s", code, e.claude.sessionStarted(), e.errOut.String())
	}
}

func TestRunWithoutANameNeverPicksARemovedAccount(t *testing.T) {
	e := newEnv(t)
	e.signIn("gone", "g@example.com")
	e.signIn("kept", "k@example.com")
	withLimits(t, e, map[string]pick.Limits{"gone": seenLimits(0, 0), "kept": seenLimits(90, 90)})
	if code := e.run("remove", "gone", "--force"); code != 0 {
		t.Fatalf("remove exited %d: %s", code, e.errOut.String())
	}
	e.reset()

	if code := e.run("run"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := ranAs(e); got != e.folder("kept") {
		t.Errorf("ran as %q, want kept", got)
	}
}

func TestRunWithoutANamePassesTheArgumentsAfterTheDoubleDashToClaude(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	if code := e.run("run", "--", "--model", "opus", "-p", "hi"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if argv, _, _ := e.claude.session(); !reflect.DeepEqual(argv, []string{"--model", "opus", "-p", "hi"}) {
		t.Errorf("claude got %v", argv)
	}
}

func TestRunWithoutANameStillRefusesAFlagOfItsOwn(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	code := e.run("run", "--model", "opus")

	if code != 2 || !strings.Contains(e.errOut.String(), "unknown flag --model") || e.claude.sessionStarted() {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}

// the rules decide what clalt chooses. a name is a choice already made
func TestRunByNameIgnoresTheFolderRules(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.setRules("work", account.Rules{Folders: []string{subfolder(t, "elsewhere")}})
	t.Chdir(subfolder(t, "here"))

	if code := e.run("run", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := ranAs(e); got != e.folder("work") {
		t.Errorf("ran as %q", got)
	}
	if strings.Contains(e.errOut.String(), "picked") {
		t.Errorf("a named run said it picked:\n%s", e.errOut.String())
	}
}

func TestRunWithoutANameDoesNotTouchTheFoldersOfTheOthers(t *testing.T) {
	e := newEnv(t)
	e.signIn("alpha", "a@example.com")
	e.signIn("beta", "b@example.com")
	before := len(e.claude.calls("status"))

	if code := e.run("run"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	// one status call to pick, and the ones a run makes for itself
	asked := e.claude.calls("status")[before:]
	for _, folder := range asked {
		if folder != e.folder("alpha") {
			t.Errorf("claude was asked about %s, which was not picked", filepath.Base(folder))
		}
	}
}

func TestHelpSaysThatRunWithoutANamePicksAnAccount(t *testing.T) {
	e := newEnv(t)

	e.run("help")

	for _, want := range []string{"clalt run [-- ...]", "the most room"} {
		if !strings.Contains(e.out.String(), want) {
			t.Errorf("the help is missing %q:\n%s", want, e.out.String())
		}
	}
}
