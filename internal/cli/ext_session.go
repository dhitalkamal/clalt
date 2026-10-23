package cli

import (
	"encoding/json"
	"os"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/defaulthome"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/fsutil"
	"github.com/dhitalkamal/clalt/internal/registry"
	"github.com/dhitalkamal/clalt/internal/usage"
)

// extCtx is what a hook of plain claude needs to know: the account its default
// home is signed in as, and whether the user turned usage attribution off.
type extCtx struct {
	account string
	off     bool
	checked time.Time
}

// extContext says whether this call is plain claude in a home that matches an
// account, from the cache only. it is what keeps the global hooks a no-op for
// everything else: a session of clalt run has CLALT_SESSION and never gets here,
// a claude in another home is not the default home, and with no match there is
// nobody to charge.
func extContext() (extCtx, bool) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" && !fsutil.SamePath(dir, config.SharedClaudeDir()) {
		return extCtx{}, false
	}
	c, ok := defaulthome.Read(config.DefaultHomePath())
	if !ok || !c.Active() {
		return extCtx{}, false
	}
	return extCtx{account: c.Account, off: c.Off, checked: c.CheckedAt}, true
}

// extHook records one hook event of plain claude under ext-<claude session id>.
// it is applyHook for a session without a registry record.
func (a *App) extHook(ctx extCtx, event string, raw []byte, deadline time.Time) {
	var in feed.HookInput
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil && event != "PostToolUse" && event != "PostToolUseFailure" {
			hookLog(a.clock(), event, "ext: the input is not json: %v", err)
			return
		}
	}
	key, ok := feed.ExtHandle(in.SessionID)
	if !ok {
		return
	}
	wait := hookLockWait
	if event == "SessionEnd" {
		wait = sessionEndLockWait
	}
	store := feed.Store{Dir: config.FeedDir(), LockWait: capWait(wait, deadline)}
	_, existed := store.Session(key)
	if !existed && (event == "PostToolUse" || event == "PostToolUseFailure") {
		// nothing to end, and no file to leave an activity note beside
		return
	}
	if !existed && a.clock().Sub(ctx.checked) > extStaleAfter {
		// the login may have changed since the cache was written, and a new session
		// is charged for good: ask again, or do not attribute it
		var ok bool
		if ctx, ok = a.freshExtContext(deadline); !ok {
			hookLog(a.clock(), event, "ext: the default home could not be checked in time, the session is not recorded")
			return
		}
	}
	rec := registry.Record{Handle: key, Account: ctx.account, Cwd: in.Cwd}
	a.applyHook(registry.Registry{Dir: config.SessionsDir(), LockWait: wait}, rec, event, in, wait, deadline)
	if _, now := store.Session(key); now && !existed {
		noteExtSession(ctx, in.SessionID, a.clock())
	}
}

// noteExtSession puts a session that clalt run did not start in the ledger, as
// attributed by the default home, unless the id has a line already or the user
// turned that off.
func noteExtSession(ctx extCtx, id string, now time.Time) {
	if ctx.off {
		return
	}
	if _, known := attributedAccount(id); known {
		return
	}
	line := usage.SessionAccount{Ts: now, SessionID: id, Account: ctx.account, Source: usage.SourceDefault}
	_ = usage.AppendSessionAccount(config.SessionAccountsPath(), line)
}

// recordStatuslineExt is recordStatusline for plain claude: the limits go to the
// matched account and the session is kept as ext-<claude session id>.
func (a *App) recordStatuslineExt(ctx extCtx, raw []byte) {
	now := a.clock()
	var in statusInput
	if err := json.Unmarshal(raw, &in); err != nil {
		hookLog(now, "statusline", "ext: the input is not json: %v", err)
		return
	}
	store := feed.Store{Dir: config.FeedDir(), LockWait: statuslineLockWait}
	key, ok := feed.ExtHandle(in.SessionID)
	_, existed := store.Session(key)
	if !ok || (!existed && now.Sub(ctx.checked) > extStaleAfter) {
		// the statusline has no time to ask claude who the default home is, and a
		// session or a limit under a stale answer may be the wrong account's. the
		// hook of the session start checks, and later calls find it recorded
		return
	}
	if err := store.MergeLimits(ctx.account, in.RateLimits.FiveHour.window(), in.RateLimits.SevenDay.window(), now); err != nil {
		hookLog(now, "statusline", "ext: record the limits: %v", err)
	}
	err := store.UpdateSession(key, ctx.account, now, func(s *feed.SessionState) bool { return applyStatus(s, in) || !existed })
	if err != nil {
		hookLog(now, "statusline", "ext: update the feed: %v", err)
		return
	}
	if !existed {
		noteExtSession(ctx, in.SessionID, now)
	}
}

// extKeepIdle is how long an ext session that nothing reported on is kept before
// pruning: claude may have been killed without a SessionEnd.
const extKeepIdle = 24 * time.Hour

// keepExt says whether the feed file of an ext session stays when pruning: it is
// not ended and it reported lately.
func keepExt(store feed.Store, handle string, now time.Time) bool {
	st, ok := store.Session(handle)
	return ok && feed.IsExt(handle) && st.State != feed.StateEnded && now.Sub(st.UpdatedAt) < extKeepIdle
}

// extStaleAfter is how old the cache of the default home may be before a new
// plain claude session makes a hook ask claude again.
var extStaleAfter = 5 * time.Minute

// freshExtContext refreshes the cache of the default home inside the hook's
// budget and returns what it says. false when the check did not finish in time
// or the home no longer matches an account. the call to claude is not cut short,
// only waited for: ending it in the middle of a token refresh can sign a login
// out, so it finishes on its own after the hook is gone.
func (a *App) freshExtContext(deadline time.Time) (extCtx, bool) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = recover() }()
		_, _ = a.refreshDefault(false)
	}()
	wait := time.Until(deadline) - 300*time.Millisecond
	if wait < 0 {
		wait = 0
	}
	select {
	case <-done:
		return extContext()
	case <-time.After(wait):
		return extCtx{}, false
	}
}
