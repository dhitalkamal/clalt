package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/registry"
)

const statusPayload = `{"session_id":"claude-9","cwd":"/w","model":{"id":"claude-opus-5","display_name":"Opus"},` +
	`"workspace":{"current_dir":"/w/sub"},"context_window":{"used_percentage":42.5},` +
	`"rate_limits":{"five_hour":{"used_percentage":23.5,"resets_at":4102444800},"seven_day":{"used_percentage":41.2,"resets_at":4103000000}}}`

// setUserStatusLine writes a statusLine with this command into the shared settings.
func setUserStatusLine(t *testing.T, e *testEnv, command string) {
	t.Helper()
	body := `{"statusLine":{"type":"command","command":` + jsonString(command) + `}}`
	if err := os.WriteFile(filepath.Join(e.home, ".claude", "settings.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (e *testEnv) statusline(payload string) int {
	e.t.Helper()
	e.app.In = strings.NewReader(payload)
	return e.run("statusline")
}

func TestStatuslinePrintsTheUsersOwnOutputUnchangedWithTheSameInput(t *testing.T) {
	e := newEnv(t)
	liveHandle(t)
	setUserStatusLine(t, e, "cat")

	if code := e.statusline(statusPayload); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if e.out.String() != statusPayload {
		t.Errorf("stdout %q", e.out.String())
	}
}

func TestStatuslineRecordsTheSessionAndTheAccountsLimits(t *testing.T) {
	e := newEnv(t)
	h := liveHandle(t)

	if code := e.statusline(statusPayload); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	st, ok := feedStore().Session(h)
	if !ok || st.Model != "claude-opus-5" || st.Cwd != "/w/sub" || st.ContextPct == nil || *st.ContextPct != 42.5 {
		t.Errorf("%+v %v", st, ok)
	}
	lim, ok := feedStore().Limits("work")
	if !ok || lim.FiveHour.UsedPct != 23.5 || lim.SevenDay.UsedPct != 41.2 || lim.FiveHour.ResetsAt.Unix() != 4102444800 {
		t.Errorf("%+v %v", lim, ok)
	}
	rec, _, _ := registry.Registry{Dir: config.SessionsDir()}.Get(h)
	if rec.ClaudeSessionID != "claude-9" {
		t.Errorf("claudeSessionId = %q", rec.ClaudeSessionID)
	}
}

func TestStatuslineWithoutLimitsOrContextKeepsWhatIsKnown(t *testing.T) {
	e := newEnv(t)
	h := liveHandle(t)
	e.statusline(statusPayload)

	e.statusline(`{"session_id":"claude-9","model":{"id":"claude-sonnet-5"},"context_window":{"used_percentage":null}}`)

	st, _ := feedStore().Session(h)
	lim, ok := feedStore().Limits("work")
	if st.Model != "claude-sonnet-5" || st.ContextPct == nil || !ok || lim.FiveHour == nil {
		t.Errorf("%+v %+v", st, lim)
	}
}

func TestStatuslineWithoutASessionOnlyPassesThrough(t *testing.T) {
	e := newEnv(t)
	t.Setenv("CLALT_SESSION", "")
	setUserStatusLine(t, e, "echo mine")

	code := e.statusline(statusPayload)

	if code != 0 || e.out.String() != "mine\n" {
		t.Errorf("exit %d out %q", code, e.out.String())
	}
	if _, err := os.Stat(config.FeedDir()); err == nil {
		t.Error("a feed was made")
	}
}

func TestStatuslineWithNoUserStatusLinePrintsNothing(t *testing.T) {
	e := newEnv(t)
	liveHandle(t)

	if code := e.statusline(statusPayload); code != 0 || e.out.Len() != 0 {
		t.Errorf("exit %d out %q", code, e.out.String())
	}
}

func TestStatuslinePassesTheUsersExitCodeOn(t *testing.T) {
	e := newEnv(t)
	setUserStatusLine(t, e, "echo partial; exit 3")

	code := e.statusline(`{}`)

	if code != 3 || e.out.String() != "partial\n" {
		t.Errorf("exit %d out %q", code, e.out.String())
	}
}

func TestStatuslineStopsASlowUserCommand(t *testing.T) {
	e := newEnv(t)
	setUserStatusLine(t, e, "sleep 10")
	old := statuslineWait
	statuslineWait = 200 * time.Millisecond
	t.Cleanup(func() { statuslineWait = old })

	start := time.Now()
	code := e.statusline(`{}`)

	if code == 0 || time.Since(start) > 3*time.Second {
		t.Errorf("exit %d after %v", code, time.Since(start))
	}
}

func TestStatuslineDoesNotRunItselfInALoop(t *testing.T) {
	e := newEnv(t)
	setUserStatusLine(t, e, `echo "chained=$CLALT_STATUSLINE_CHAINED"`)
	e.statusline(`{}`)
	if e.out.String() != "chained=1\n" {
		t.Fatalf("the child did not get the guard: %q", e.out.String())
	}
	e.reset()
	t.Setenv("CLALT_STATUSLINE_CHAINED", "1")

	if code := e.statusline(`{}`); code != 0 || e.out.Len() != 0 {
		t.Errorf("a chained call ran the user command again: exit %d out %q", code, e.out.String())
	}
}

func TestStatuslineFindsTheAccountThroughTheConfigDirWhenTheRecordIsGone(t *testing.T) {
	e := newEnv(t)
	t.Setenv("CLALT_SESSION", registry.NewHandle())
	t.Setenv("CLAUDE_CONFIG_DIR", accountDir(t, "work"))

	e.statusline(statusPayload)

	if _, ok := feedStore().Limits("work"); !ok {
		t.Error("the limits were not recorded")
	}
}

func TestStatuslineIgnoresNonsenseInputButStillPassesThrough(t *testing.T) {
	e := newEnv(t)
	liveHandle(t)
	setUserStatusLine(t, e, "echo ok")

	if code := e.statusline("not json"); code != 0 || e.out.String() != "ok\n" {
		t.Errorf("exit %d out %q", code, e.out.String())
	}
}

func TestStatuslineDoesNotRewriteTheSessionWhenNothingChanged(t *testing.T) {
	e := newEnv(t)
	h := liveHandle(t)
	e.statusline(statusPayload)
	path := filepath.Join(config.FeedDir(), "sessions", h+".json")
	before, _ := os.Stat(path)
	time.Sleep(20 * time.Millisecond)

	e.statusline(statusPayload)

	if after, _ := os.Stat(path); !after.ModTime().Equal(before.ModTime()) {
		t.Error("the file was rewritten for the same values")
	}
}
