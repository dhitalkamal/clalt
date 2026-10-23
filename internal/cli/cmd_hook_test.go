package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/fslock"
	"github.com/dhitalkamal/clalt/internal/registry"
)

// liveHandle makes a registry record for a session of account work whose
// wrapper is the test process, and puts its handle in the environment as
// clalt run would.
func liveHandle(t *testing.T) string {
	t.Helper()
	h := registry.NewHandle()
	err := registry.Registry{Dir: config.SessionsDir()}.Create(registry.Record{
		Handle: h, Account: "work", Folder: "/f", WrapperPID: os.Getpid(), Cwd: "/c", StartedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLALT_SESSION", h)
	return h
}

// hook runs `clalt hook <event>` with payload on stdin.
func (e *testEnv) hook(event, payload string) int {
	e.t.Helper()
	e.app.In = strings.NewReader(payload)
	return e.run("hook", event)
}

func feedStore() feed.Store { return feed.Store{Dir: config.FeedDir()} }

func TestHookWithoutASessionDoesNothingAndSaysNothing(t *testing.T) {
	e := newEnv(t)
	t.Setenv("CLALT_SESSION", "")

	code := e.hook("SessionStart", `{"session_id":"s","model":"m"}`)

	if code != 0 || e.out.Len() != 0 || e.errOut.Len() != 0 {
		t.Errorf("exit %d out %q err %q", code, e.out.String(), e.errOut.String())
	}
	if _, err := os.Stat(config.FeedDir()); err == nil {
		t.Error("a feed was made")
	}
}

func TestHookRefusesAHandleThatIsAPathAndStaysQuiet(t *testing.T) {
	e := newEnv(t)
	t.Setenv("CLALT_SESSION", "../../x")

	if code := e.hook("Stop", `{}`); code != 0 || e.out.Len() != 0 || e.errOut.Len() != 0 {
		t.Errorf("exit %d out %q err %q", code, e.out.String(), e.errOut.String())
	}
	if _, err := os.Stat(config.FeedDir()); err == nil {
		t.Error("a feed was made")
	}
}

func TestHookOfAnUnknownSessionDoesNothing(t *testing.T) {
	e := newEnv(t)
	t.Setenv("CLALT_SESSION", registry.NewHandle())

	if code := e.hook("Stop", `{}`); code != 0 || e.out.Len() != 0 {
		t.Errorf("exit %d out %q", code, e.out.String())
	}
	if _, err := os.Stat(config.FeedDir()); err == nil {
		t.Error("a feed was made for a session nobody started")
	}
}

func TestHookSessionStartRecordsTheClaudeSessionAndTheModel(t *testing.T) {
	e := newEnv(t)
	h := liveHandle(t)

	code := e.hook("SessionStart", `{"session_id":"claude-1","cwd":"/work","model":"claude-opus-5","source":"startup","hook_event_name":"SessionStart"}`)

	if code != 0 || e.out.Len() != 0 || e.errOut.Len() != 0 {
		t.Fatalf("exit %d out %q err %q", code, e.out.String(), e.errOut.String())
	}
	st, ok := feedStore().Session(h)
	if !ok || st.State != feed.StateIdle || st.Model != "claude-opus-5" || st.Cwd != "/work" || st.Account != "work" {
		t.Errorf("%+v %v", st, ok)
	}
	rec, _, _ := registry.Registry{Dir: config.SessionsDir()}.Get(h)
	if rec.ClaudeSessionID != "claude-1" {
		t.Errorf("claudeSessionId = %q", rec.ClaudeSessionID)
	}
}

func TestHookFollowsAConversationThatGetsANewId(t *testing.T) {
	e := newEnv(t)
	h := liveHandle(t)
	e.hook("SessionStart", `{"session_id":"one"}`)

	e.hook("SessionEnd", `{"session_id":"one","reason":"clear"}`)
	e.hook("SessionStart", `{"session_id":"two","source":"clear"}`)

	rec, _, _ := registry.Registry{Dir: config.SessionsDir()}.Get(h)
	st, _ := feedStore().Session(h)
	if rec.ClaudeSessionID != "two" || st.State != feed.StateIdle {
		t.Errorf("id %q state %s", rec.ClaudeSessionID, st.State)
	}
}

func TestHookWalksATurn(t *testing.T) {
	e := newEnv(t)
	h := liveHandle(t)
	steps := []struct {
		event, payload string
		want           feed.State
		reason         string
	}{
		{"SessionStart", `{"session_id":"s"}`, feed.StateIdle, ""},
		{"UserPromptSubmit", `{"session_id":"s","prompt":"do it"}`, feed.StateBusy, ""},
		{"Notification", `{"session_id":"s","notification_type":"permission_prompt","message":"needs approval"}`, feed.StateNeedsYou, "permission_prompt"},
		{"PostToolUse", `{"session_id":"s","tool_name":"Bash"}`, feed.StateBusy, ""},
		{"Stop", `{"session_id":"s"}`, feed.StateIdle, ""},
		{"SessionEnd", `{"session_id":"s","reason":"other"}`, feed.StateEnded, ""},
	}
	for _, s := range steps {
		if code := e.hook(s.event, s.payload); code != 0 {
			t.Fatalf("%s exit %d", s.event, code)
		}
		st, _ := feedStore().Session(h)
		if st.State != s.want || st.Reason != s.reason {
			t.Errorf("after %s: %s/%q, want %s/%q", s.event, st.State, st.Reason, s.want, s.reason)
		}
	}
	if e.out.Len() != 0 || e.errOut.Len() != 0 {
		t.Errorf("a hook printed: out %q err %q", e.out.String(), e.errOut.String())
	}
}

func TestHookToolResultWhileNotNeedingYouWritesNothing(t *testing.T) {
	e := newEnv(t)
	h := liveHandle(t)
	e.hook("UserPromptSubmit", `{"session_id":"s"}`)
	path := filepath.Join(config.FeedDir(), "sessions", h+".json")
	before, _ := os.Stat(path)
	time.Sleep(20 * time.Millisecond)

	e.hook("PostToolUse", `{"session_id":"s","tool_response":{"big":"`+strings.Repeat("x", 2<<20)+`"}}`)

	after, _ := os.Stat(path)
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("the file was rewritten")
	}
}

func TestHookStopFailureKeepsTheRateLimitAsTheLastError(t *testing.T) {
	e := newEnv(t)
	h := liveHandle(t)
	e.hook("UserPromptSubmit", `{}`)

	e.hook("StopFailure", `{"error":"rate_limit","error_details":"429"}`)

	st, _ := feedStore().Session(h)
	if st.State != feed.StateIdle || st.LastError != "rate_limit" {
		t.Errorf("%+v", st)
	}
}

func TestHookLogsBrokenInputToTheLogAndStillExitsZero(t *testing.T) {
	e := newEnv(t)
	liveHandle(t)

	code := e.hook("Stop", `{not json`)

	if code != 0 || e.out.Len() != 0 || e.errOut.Len() != 0 {
		t.Fatalf("exit %d out %q err %q", code, e.out.String(), e.errOut.String())
	}
	raw, err := os.ReadFile(filepath.Join(config.LogsDir(), "hook.log"))
	if err != nil || !strings.Contains(string(raw), "Stop") {
		t.Errorf("log %q %v", raw, err)
	}
	if _, perr := time.Parse(time.RFC3339, strings.Fields(string(raw))[0]); perr != nil {
		t.Errorf("a log line starts with a time: %v", perr)
	}
}

func TestHookGivesUpQuicklyWhenTheFeedLockIsHeld(t *testing.T) {
	e := newEnv(t)
	liveHandle(t)
	if err := os.MkdirAll(config.FeedDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	l, err := fslock.Acquire(filepath.Join(config.FeedDir(), ".lock"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()

	start := time.Now()
	code := e.hook("UserPromptSubmit", `{}`)

	if code != 0 || time.Since(start) > 2*time.Second {
		t.Errorf("exit %d after %v", code, time.Since(start))
	}
	raw, _ := os.ReadFile(filepath.Join(config.LogsDir(), "hook.log"))
	if !strings.Contains(string(raw), "UserPromptSubmit") {
		t.Errorf("the timeout was not logged: %q", raw)
	}
}

func TestHookSessionStartPrunesFeedFilesOfSessionsThatAreGone(t *testing.T) {
	e := newEnv(t)
	liveHandle(t)
	gone := registry.NewHandle()
	_ = feedStore().UpdateSession(gone, "work", time.Now().Add(-time.Hour), func(s *feed.SessionState) bool { return true })

	e.hook("SessionStart", `{"session_id":"s"}`)

	if _, ok := feedStore().Session(gone); ok {
		t.Error("the feed file of a gone session is still there")
	}
}

func TestHookNeverPanicsOnAMissingEvent(t *testing.T) {
	e := newEnv(t)
	liveHandle(t)
	e.app.In = strings.NewReader(`{}`)
	if code := e.run("hook"); code != 0 {
		t.Errorf("exit %d", code)
	}
}

func TestAPermissionNotificationRightAfterAToolResultIsLate(t *testing.T) {
	e := newEnv(t)
	h := liveHandle(t)
	e.hook("UserPromptSubmit", `{}`)
	e.hook("PostToolUse", `{"tool_name":"Bash"}`)

	e.hook("Notification", `{"notification_type":"permission_prompt"}`)

	if st, _ := feedStore().Session(h); st.State != feed.StateBusy {
		t.Errorf("state %s: the prompt was answered before the notification came", st.State)
	}
}

func TestHookDrainsItsInputEvenWhenItIsNotAboutASession(t *testing.T) {
	e := newEnv(t)
	t.Setenv("CLALT_SESSION", "")
	in := strings.NewReader(strings.Repeat("x", 3<<20))
	e.app.In = in

	e.run("hook", "PostToolUse")

	if in.Len() != 0 {
		t.Errorf("%d bytes left unread: claude's write would meet a closed pipe", in.Len())
	}
}
