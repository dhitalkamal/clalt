package cli

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
)

func defaultHomeLines(t *testing.T, e *testEnv) map[string]checkResult {
	t.Helper()
	e.reset()
	e.run("doctor", "--json")
	var rep doctorReport
	if err := json.Unmarshal(e.out.Bytes(), &rep); err != nil {
		t.Fatalf("%v: %s", err, e.out.String())
	}
	out := map[string]checkResult{}
	for _, c := range rep.Checks {
		if c.Scope == "default home" {
			out[c.Check] = c
		}
	}
	return out
}

func TestDoctorSaysWhichAccountPlainClaudeIs(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.plainClaudeAs("W@example.com")
	e.app.refreshDefaultHome()

	got := defaultHomeLines(t, e)

	if c := got["match"]; c.Status != statusOK || !strings.Contains(c.Detail, "work") || !strings.Contains(c.Detail, "~/.claude") {
		t.Errorf("match: %+v", c)
	}
	if runtime.GOOS == "windows" {
		// the feed is refused there, so doctor does not offer it
		if c, ok := got["feed"]; ok {
			t.Errorf("feed offered on windows: %+v", c)
		}
		return
	}
	if c := got["feed"]; c.Status != statusOK || !strings.Contains(c.Detail, "clalt feed install") {
		t.Errorf("feed: %+v", c)
	}
}

func TestDoctorSaysPlainClaudeStaysOutsideAndWhy(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.plainClaudeAs("someone@else.com")
	e.app.refreshDefaultHome()

	got := defaultHomeLines(t, e)

	if c := got["match"]; c.Status != statusOK || !strings.Contains(c.Detail, "outside") || !strings.Contains(c.Detail, "someone@else.com") {
		t.Errorf("match: %+v", c)
	}
	if _, ok := got["feed"]; ok {
		t.Error("a feed line without a match")
	}
}

func TestDoctorAsksClaudeNothingAboutPlainClaudeAndSaysWhenItWasNotChecked(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.plainClaudeAs("w@example.com")
	before := len(e.claude.calls("status"))

	got := defaultHomeLines(t, e)

	if c := got["match"]; c.Status != statusOK || !strings.Contains(c.Detail, "not been checked") {
		t.Errorf("match: %+v", c)
	}
	if n := len(e.claude.calls("status")); n != before+1 {
		t.Errorf("status calls went from %d to %d: doctor asked about the default home", before, n)
	}
}

func TestDoctorTakesPlainClaudeSessionsAsAliveNotAsLeftovers(t *testing.T) {
	e := newEnv(t)
	h, _ := feed.ExtHandle("plain-1")
	err := feedStore().UpdateSession(h, "work", time.Now(), func(s *feed.SessionState) bool { s.State = feed.StateIdle; return true })
	if err != nil {
		t.Fatal(err)
	}

	if got := feedLines(t, e)["sessions"]; got.Status != statusOK {
		t.Errorf("%+v", got)
	}
}
