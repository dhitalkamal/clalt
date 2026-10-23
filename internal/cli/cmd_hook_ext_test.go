package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/defaulthome"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
	"github.com/dhitalkamal/clalt/internal/usage"
)

// matchDefaultHome writes the cache that says plain claude is signed in as
// account work.
func matchDefaultHome(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(config.ClaltDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	c := defaulthome.Cache{State: defaulthome.Matched, Account: "work", Email: "w@example.com", CheckedAt: time.Now()}
	if err := defaulthome.Write(config.DefaultHomePath(), c); err != nil {
		t.Fatal(err)
	}
}

func extKey(id string) string { h, _ := feed.ExtHandle(id); return h }

func quiet(t *testing.T, e *testEnv, code int) {
	t.Helper()
	if code != 0 || e.out.Len() != 0 || e.errOut.Len() != 0 {
		t.Errorf("exit %d out %q err %q", code, e.out.String(), e.errOut.String())
	}
}

func TestHookOutsideAMatchedHomeDoesNothingAndPrintsNothing(t *testing.T) {
	e := newEnv(t)
	t.Setenv("CLALT_SESSION", "")

	// no cache at all
	quiet(t, e, e.hook("SessionStart", `{"session_id":"abc-1","model":"m"}`))
	// plain claude known and outside
	if err := os.MkdirAll(config.ClaltDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := defaulthome.Write(config.DefaultHomePath(), defaulthome.Cache{State: defaulthome.Outside, Reason: "x", CheckedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	quiet(t, e, e.hook("SessionStart", `{"session_id":"abc-1","model":"m"}`))

	if _, err := os.Stat(config.FeedDir()); err == nil {
		t.Error("a feed was made")
	}
	if _, err := os.Stat(config.SessionAccountsPath()); err == nil {
		t.Error("the ledger was written")
	}
}

func TestHookInAnotherClaudeHomeIsNotThePlainClaudeEvenWhenMatched(t *testing.T) {
	e := newEnv(t)
	t.Setenv("CLALT_SESSION", "")
	matchDefaultHome(t)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(config.AccountsDir(), "work"))

	quiet(t, e, e.hook("SessionStart", `{"session_id":"abc-1"}`))

	if _, err := os.Stat(config.FeedDir()); err == nil {
		t.Error("a feed was made")
	}
}

func TestHookOfPlainClaudeInAMatchedHomeFeedsAnExtSession(t *testing.T) {
	e := newEnv(t)
	t.Setenv("CLALT_SESSION", "")
	matchDefaultHome(t)

	quiet(t, e, e.hook("SessionStart", `{"session_id":"abc-1","cwd":"/p","model":"claude-opus-5","source":"startup"}`))
	quiet(t, e, e.hook("UserPromptSubmit", `{"session_id":"abc-1"}`))

	st, ok := feedStore().Session(extKey("abc-1"))
	if !ok || st.Account != "work" || st.State != feed.StateBusy || st.Cwd != "/p" || st.Model != "claude-opus-5" {
		t.Fatalf("%+v %v", st, ok)
	}
	// one ledger line, from the first sight of the id
	lines, _ := usage.ReadSessionAccounts(config.SessionAccountsPath())
	if len(lines) != 1 || lines[0].SessionID != "abc-1" || lines[0].Account != "work" || lines[0].Source != usage.SourceDefault {
		t.Errorf("ledger %+v", lines)
	}
	quiet(t, e, e.hook("SessionEnd", `{"session_id":"abc-1","reason":"prompt_input_exit"}`))
	if st, _ := feedStore().Session(extKey("abc-1")); st.State != feed.StateEnded {
		t.Errorf("state %s", st.State)
	}
}

func TestHookWithAnIdThatIsNotSafeIsIgnored(t *testing.T) {
	e := newEnv(t)
	t.Setenv("CLALT_SESSION", "")
	matchDefaultHome(t)

	quiet(t, e, e.hook("SessionStart", `{"session_id":"../../x"}`))
	quiet(t, e, e.hook("SessionStart", `{}`))

	if _, err := os.Stat(config.FeedDir()); err == nil {
		t.Error("a feed was made")
	}
}

func TestHookDoesNotRecordTwiceWhenBothInstallsAreActive(t *testing.T) {
	e := newEnv(t)
	h := liveHandle(t)
	matchDefaultHome(t)

	// the per-session hook and the global one run the same command in the same
	// environment, so the same call twice is what claude does
	quiet(t, e, e.hook("SessionStart", `{"session_id":"claude-1"}`))
	quiet(t, e, e.hook("SessionStart", `{"session_id":"claude-1"}`))

	all, _ := feedStore().Sessions()
	if len(all) != 1 || all[0].Handle != h {
		t.Errorf("sessions: %+v", all)
	}
	if _, ok := feedStore().Session(extKey("claude-1")); ok {
		t.Error("an ext record was made beside the clalt run one")
	}
	lines, _ := usage.ReadSessionAccounts(config.SessionAccountsPath())
	if len(lines) != 1 || lines[0].Source != "" {
		t.Errorf("ledger %+v", lines)
	}
}

func TestStatuslineOfPlainClaudeMergesLimitsIntoTheMatchedAccount(t *testing.T) {
	skipWindows(t, "the user command runs through the platform shell, and these use sh syntax")
	e := newEnv(t)
	t.Setenv("CLALT_SESSION", "")
	matchDefaultHome(t)
	setUserStatusLine(t, e, "echo mine")

	code := e.statusline(statusPayload)

	if code != 0 || e.out.String() != "mine\n" {
		t.Errorf("exit %d out %q", code, e.out.String())
	}
	l, ok := feedStore().Limits("work")
	if !ok || l.FiveHour == nil || l.FiveHour.UsedPct != 23.5 {
		t.Errorf("limits %+v %v", l, ok)
	}
	st, ok := feedStore().Session(extKey("claude-9"))
	if !ok || st.Account != "work" || st.Model != "claude-opus-5" || st.ContextPct == nil {
		t.Errorf("session %+v %v", st, ok)
	}
}

func TestStatuslineOutsideAMatchedHomeOnlyPassesThrough(t *testing.T) {
	skipWindows(t, "the user command runs through the platform shell, and these use sh syntax")
	e := newEnv(t)
	t.Setenv("CLALT_SESSION", "")
	setUserStatusLine(t, e, "echo mine")

	e.statusline(statusPayload)

	if e.out.String() != "mine\n" {
		t.Errorf("out %q", e.out.String())
	}
	if _, err := os.Stat(config.FeedDir()); err == nil {
		t.Error("a feed was made")
	}
}

func TestPruneKeepsFreshExtSessionsAndDropsOldOnes(t *testing.T) {
	newEnv(t)
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	store := feedStore()
	mk := func(id string, state feed.State, age time.Duration) string {
		h := extKey(id)
		err := store.UpdateSession(h, "work", now.Add(-age), func(s *feed.SessionState) bool { s.State = state; return true })
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	fresh := mk("fresh", feed.StateIdle, 3*time.Hour)
	stale := mk("stale", feed.StateIdle, 25*time.Hour)
	ended := mk("ended", feed.StateEnded, time.Hour)

	pruneFeed(store, registry.Registry{Dir: config.SessionsDir()}, now)

	for h, want := range map[string]bool{fresh: true, stale: false, ended: false} {
		if _, ok := store.Session(h); ok != want {
			t.Errorf("%s present=%v want %v", h, ok, want)
		}
	}
}

func TestAToolEventOfAnUnknownPlainClaudeSessionLeavesNothingBehind(t *testing.T) {
	e := newEnv(t)
	t.Setenv("CLALT_SESSION", "")
	matchDefaultHome(t)

	quiet(t, e, e.hook("PostToolUse", `{"session_id":"abc-1"}`))

	if _, err := os.Stat(config.FeedDir()); err == nil {
		t.Error("a feed was made")
	}
}

// staleMatch is a cache that says plain claude is the account work, checked
// long ago
func staleMatch(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(config.ClaltDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-3 * time.Hour)
	c := defaulthome.Cache{State: defaulthome.Matched, Account: "work", CheckedAt: old, MatchedSince: old}
	if err := defaulthome.Write(config.DefaultHomePath(), c); err != nil {
		t.Fatal(err)
	}
}

func TestANewPlainClaudeSessionWithAStaleCacheIsCheckedFirst(t *testing.T) {
	e := newEnv(t)
	t.Setenv("CLALT_SESSION", "")
	e.signIn("work", "a@example.com")
	e.signIn("side", "b@example.com")
	staleMatch(t)
	e.plainClaudeAs("b@example.com") // the login changed since the cache was written

	quiet(t, e, e.hook("SessionStart", `{"session_id":"abc-1"}`))

	st, ok := feedStore().Session(extKey("abc-1"))
	if !ok || st.Account != e.account("side").Name {
		t.Fatalf("%+v %v", st, ok)
	}
	if lines, _ := usage.ReadSessionAccounts(config.SessionAccountsPath()); len(lines) != 1 || lines[0].Account != st.Account {
		t.Errorf("ledger %+v", lines)
	}
}

func TestANewPlainClaudeSessionIsNotAttributedWhenTheCheckCannotFinishInTime(t *testing.T) {
	e := newEnv(t)
	t.Setenv("CLALT_SESSION", "")
	e.signIn("work", "a@example.com")
	e.plainClaudeAs("a@example.com")
	staleMatch(t)
	e.claude.set("STATUS_HANGS", "1")
	old := hookBudget
	hookBudget = 800 * time.Millisecond
	t.Cleanup(func() { hookBudget = old })

	quiet(t, e, e.hook("SessionStart", `{"session_id":"abc-1"}`))

	if _, ok := feedStore().Session(extKey("abc-1")); ok {
		t.Error("a session was recorded on a guess")
	}
	if _, err := os.Stat(config.SessionAccountsPath()); err == nil {
		t.Error("the ledger was written on a guess")
	}
}

func TestAKnownPlainClaudeSessionDoesNotAskClaudeAgain(t *testing.T) {
	e := newEnv(t)
	t.Setenv("CLALT_SESSION", "")
	matchDefaultHome(t)
	quiet(t, e, e.hook("SessionStart", `{"session_id":"abc-1"}`))
	staleMatch(t)
	e.claude.set("STATUS_HANGS", "1")

	start := time.Now()
	quiet(t, e, e.hook("UserPromptSubmit", `{"session_id":"abc-1"}`))

	if time.Since(start) > 3*time.Second {
		t.Error("the hook waited for claude")
	}
}

func TestStatuslineOfAnUnknownPlainClaudeSessionWaitsForTheHookWhenTheCacheIsStale(t *testing.T) {
	skipWindows(t, "the user command runs through the platform shell, and these use sh syntax")
	e := newEnv(t)
	t.Setenv("CLALT_SESSION", "")
	staleMatch(t)

	e.statusline(statusPayload)

	if _, err := os.Stat(config.FeedDir()); err == nil {
		t.Error("the statusline recorded under a stale answer")
	}
}
