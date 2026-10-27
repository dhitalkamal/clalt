package dashboard

import (
	"strings"
	"testing"
	"time"
)

func TestProfileRows(t *testing.T) {
	lines := linesOf(profileContent(ctxFor(wideBody), testAccount()))
	rows := []string{
		"Folder     ~/.config/clalt/profiles/robin",
		"Claude     2.1.296",
		"Settings   shared with ~/.claude (skills, agents, MCP)",
		"Login      signed in 7 days ago, re-login due in 23 days",
		"Usage feed installed, last reading 4s ago",
		"Last used  4m ago in ~/Projects/personal/clalt",
	}
	for _, r := range rows {
		if lineWith(lines, r) == "" {
			t.Errorf("missing profile row %q in\n%s", r, strings.Join(lines, "\n"))
		}
	}
}

func TestProfileVariants(t *testing.T) {
	a := testAccount()
	a.Profile.SharedSettings = false
	a.Profile.FeedInstalled = false
	a.Profile.ClaudeVersion = ""
	a.Login = Login{SignedInAt: testNow, DueAt: testNow.Add(48 * time.Hour)}
	a.LastUsedAt = time.Time{}
	c := profileContent(ctxFor(wideBody), a)
	out := joined(c)
	for _, want := range []string{
		"Settings   own copy, not shared with ~/.claude",
		"Usage feed not installed",
		"Claude     not found",
		"Login      signed in today, re-login due in 2 days",
		"Last used  never",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in\n%s", want, out)
		}
	}
	if sp := spanContaining(c, "installed"); sp.t != toneWarn {
		t.Errorf("a missing feed is a warning, got tone %d", sp.t)
	}
	if sp := spanContaining(c, "re-login"); sp.t != toneWarn {
		t.Errorf("two days left is a warning, got tone %d", sp.t)
	}
}

func TestProfileWrapsLongValues(t *testing.T) {
	a := testAccount()
	a.Profile.Folder = "~/.config/clalt/profiles/a-very-long-profile-folder-name-that-needs-wrapping"
	lines := linesOf(profileContent(ctxFor(43), a))
	for i, l := range lines {
		if len(l) > 43 {
			t.Errorf("line %d is %d wide: %q", i, len(l), l)
		}
	}
	if len(lines) < 7 {
		t.Errorf("the long folder wraps onto another line:\n%s", strings.Join(lines, "\n"))
	}
}

func TestProfileSignedOut(t *testing.T) {
	if got := linesOf(profileContent(ctxFor(wideBody), signedOutAccount())); len(got) != 1 || got[0] != "Signed out. Press s to sign in." {
		t.Errorf("signed out profile = %q", got)
	}
}

func TestProfileAgesFollowTheClockAndUnknownTimesAreLeftOut(t *testing.T) {
	c := ctxFor(wideBody)
	c.now = testNow.Add(2 * time.Hour)
	out := joined(profileContent(c, testAccount()))
	for _, want := range []string{"signed in 7 days ago", "last reading 2h ago", "Last used  2h ago in"} {
		if !strings.Contains(out, want) {
			t.Errorf("two hours later: want %q in\n%s", want, out)
		}
	}
	a := testAccount()
	a.Login, a.Limits.ReadAt = Login{}, time.Time{}
	out = joined(profileContent(ctxFor(wideBody), a))
	for _, want := range []string{"Login      signed in\n", "installed, no reading yet"} {
		if !strings.Contains(out, want) {
			t.Errorf("unknown times: want %q in\n%s", want, out)
		}
	}
}

func TestProfileSaysWhenTheAccountIsAlsoTheDefaultClaude(t *testing.T) {
	a := testAccount()
	if strings.Contains(joined(profileContent(ctxFor(wideBody), a)), "default claude") {
		t.Error("the default claude is mentioned for an account that is not")
	}
	a.Profile.DefaultHome = true

	out := joined(profileContent(ctxFor(wideBody), a))

	if !strings.Contains(out, "Default    also the default claude (~/.claude)") {
		t.Errorf("missing the row in\n%s", out)
	}
}
