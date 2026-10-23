package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
)

func TestRunInstallsTheHooksAndTheStatusLineWithTheRunningBinary(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")

	if code := e.run("run", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	exe, _ := os.Executable()
	p := parseSettings(t, e.claude.sessionSettings())
	if got := p.Hooks["Stop"][0].Hooks[0].Command; got != shQuote(exe)+" hook Stop" {
		t.Errorf("Stop hook runs %q", got)
	}
	if p.StatusLine["command"] != shQuote(exe)+" statusline" {
		t.Errorf("statusLine %v", p.StatusLine)
	}
}

func TestRunKeepsHowTheUsersStatusLineIsDrawn(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	body := `{"statusLine":{"type":"command","command":"x","padding":3}}`
	if err := os.WriteFile(filepath.Join(e.home, ".claude", "settings.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	e.run("run", "work")

	if got := parseSettings(t, e.claude.sessionSettings()).StatusLine["padding"]; got != float64(3) {
		t.Errorf("padding = %v", got)
	}
}

func TestRunLeavesAUsersOwnSettingsFlagAloneAndSaysThereIsNoFeed(t *testing.T) {
	for _, args := range [][]string{{"--settings", "mine.json"}, {"--settings=mine.json"}} {
		e := newEnv(t)
		e.signIn("work", "a@example.com")

		code := e.run(append([]string{"run", "work", "--"}, args...)...)

		argv, _, _ := e.claude.session()
		if code != 0 || strings.Join(argv, " ") != strings.Join(args, " ") || e.claude.sessionSettings() != "" {
			t.Errorf("exit %d argv %v", code, argv)
		}
		if !strings.Contains(e.errOut.String(), "feed is off") {
			t.Errorf("no note about it: %s", e.errOut.String())
		}
	}
}

func TestRunKeepsTheUsersSettingsFileUntouched(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "a@example.com")
	path := filepath.Join(e.home, ".claude", "settings.json")
	before, _ := os.ReadFile(path)

	e.run("run", "work")

	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Errorf("settings.json changed: %s", after)
	}
}

func TestRunMarksTheSessionStartingWhileClaudeRunsAndEndedAfter(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	extendFake(t, e, `cat "$XDG_CONFIG_HOME"/clalt/feed/sessions/*.json > "$dir/session.feed"`)

	if code := e.run("run", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	h := runningRecord(t, e).Handle
	during, _ := os.ReadFile(filepath.Join(e.claude.dir, "session.feed"))
	if !strings.Contains(string(during), `"state":"starting"`) || !strings.Contains(string(during), h) {
		t.Errorf("feed while claude ran: %q", during)
	}
	st, ok := feedStore().Session(h)
	if !ok || st.State != feed.StateEnded {
		t.Errorf("after the run: %+v %v", st, ok)
	}
}

func TestRunPrunesTheFeedOfSessionsThatAreGone(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	gone := registry.NewHandle()
	_ = feedStore().UpdateSession(gone, "work", time.Now().Add(-time.Hour), func(*feed.SessionState) bool { return true })

	e.run("run", "work")

	if _, ok := feedStore().Session(gone); ok {
		t.Error("the feed file of a gone session survived a run")
	}
}

func TestRunStartsWhenTheFeedCannotBeWritten(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	if err := os.MkdirAll(filepath.Dir(config.FeedDir()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.FeedDir(), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if code := e.run("run", "work"); code != 0 || !e.claude.sessionStarted() {
		t.Errorf("exit %d started %v: %s", code, e.claude.sessionStarted(), e.errOut.String())
	}
}
