package cli

import (
	"strings"
	"testing"
)

// what claude says about a corrupt .claude.json that it set aside
const corruptNote = "Warning: ~/.claude.json was corrupt, a copy is in ~/.claude.json.corrupted.17"

// claude writes a line to stderr next to its answer when it set a corrupt
// .claude.json aside. the account is fine now, but whatever was in that file is
// not, so doctor shows the line as a warning on the account it came from.
func TestDoctorShowsWhatClaudeWarnedAboutForThatAccountOnly(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	e.signIn("home", "h@example.com")
	e.claude.warnAbout(e.folder("work"), corruptNote)

	code, rep := doctor(t, e)

	rep.want(t, "work", "claudeWarning", "warn", corruptNote)
	if _, ok := rep.find("home", "claudeWarning"); ok {
		t.Errorf("home has a claudeWarning check: %+v", rep.Checks)
	}
	if code != 0 || rep.Failed != 0 || rep.Warned != 1 {
		t.Errorf("exit %d, %d failed, %d warned, want 0, 0, 1: %+v", code, rep.Failed, rep.Warned, rep.Checks)
	}
}

func TestDoctorShowsTheWarningOfASignedOutAccountToo(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	e.claude.signOut(e.folder("work"))
	e.claude.warnAbout(e.folder("work"), corruptNote)

	code, rep := doctor(t, e)

	rep.want(t, "work", "signedIn", "warn", "clalt login work")
	rep.want(t, "work", "claudeWarning", "warn", corruptNote)
	if code != 0 || rep.Failed != 0 {
		t.Errorf("exit %d, %d failed: %+v", code, rep.Failed, rep.Checks)
	}
}

// the line is claude's text, so it goes through the same cleaning as the rest
func TestTheWarningOfClaudeIsCleanedBeforeDoctorShowsIt(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	e.claude.warnAbout(e.folder("work"), "bad \x1b[31mred\x1b[0m \x1b]0;owned\x07 \u202egnp.exe")

	_, rep := doctor(t, e)

	c, ok := rep.find("work", "claudeWarning")
	if !ok {
		t.Fatalf("no claudeWarning check: %+v", rep.Checks)
	}
	if hasControl(c.Detail) || strings.ContainsRune(c.Detail, '\u202e') || !strings.Contains(c.Detail, `\x1b[31mred`) {
		t.Errorf("detail: %q", c.Detail)
	}
}

// node prints notices of its own before the real output. they are not claude
// saying something about the account, and a healthy install stays quiet.
func TestDoctorIgnoresTheNoiseOfNodeOnStderr(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	e.claude.warnAbout(e.folder("work"), "(node:4242) [DEP0040] DeprecationWarning: The `punycode` module is deprecated.\n"+
		"(Use `node --trace-deprecation ...` to show where the warning was created)")

	code, rep := doctor(t, e)

	if _, ok := rep.find("work", "claudeWarning"); ok {
		t.Errorf("node noise became a check: %+v", rep.Checks)
	}
	if code != 0 || rep.Warned != 0 || rep.Failed != 0 {
		t.Errorf("exit %d, %d failed, %d warned: %+v", code, rep.Failed, rep.Warned, rep.Checks)
	}
}

// a status call that failed has its stderr in the error, and that is already
// the warning on the signedIn check
func TestAWarningThatComesWithAFailedStatusIsShownOnce(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	e.claude.warnAbout(e.folder("work"), corruptNote)
	e.claude.set("STATUS_BROKEN", "1")

	_, rep := doctor(t, e)

	rep.want(t, "work", "signedIn", "warn", corruptNote)
	if _, ok := rep.find("work", "claudeWarning"); ok {
		t.Errorf("the warning is shown twice: %+v", rep.Checks)
	}
}
