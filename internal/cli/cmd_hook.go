package cli

import (
	"encoding/json"
	"os"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
	"github.com/dhitalkamal/clalt/internal/usage"
)

// cmdHook is `clalt hook <event>`, which claude runs for the hooks that clalt
// run installs. it must never print (the output of some hooks goes into
// claude's context), never fail and never stall, so it always returns nil, gives
// up on a lock after 300 ms and writes what went wrong to the hook log. without
// a handle in CLALT_SESSION it is not about a clalt session and does nothing.
func (a *App) cmdHook(args []string) (err error) {
	event := ""
	if len(args) > 0 {
		event = args[0]
	}
	defer func() {
		if r := recover(); r != nil {
			hookLog(a.clock(), event, "panic: %v", r)
		}
		err = nil
	}()
	// read first, so that claude's write never meets a closed pipe
	raw := readInput(a.In)
	handle := os.Getenv(sessionEnvVar)
	if !registry.ValidHandle(handle) {
		return nil
	}
	if event == "" {
		hookLog(a.clock(), "hook", "no event name was given")
		return nil
	}
	wait := hookLockWait
	if event == "SessionEnd" {
		wait = sessionEndLockWait
	}
	reg := registry.Registry{Dir: config.SessionsDir(), LockWait: wait}
	rec, ok, rerr := reg.Get(handle)
	if rerr != nil {
		hookLog(a.clock(), event, "%s: read the session record: %v", handle, rerr)
		return nil
	}
	if !ok {
		return nil
	}
	var in feed.HookInput
	if len(raw) > 0 {
		if jerr := json.Unmarshal(raw, &in); jerr != nil && event != "PostToolUse" && event != "PostToolUseFailure" {
			hookLog(a.clock(), event, "%s: the input is not json: %v", handle, jerr)
			return nil
		}
	}
	a.applyHook(reg, rec, event, in, wait)
	return nil
}

// applyHook records one event of a session: its state in the feed, and the
// claude session id in the registry when it changed (a /clear or /resume makes
// a new one).
func (a *App) applyHook(reg registry.Registry, rec registry.Record, event string, in feed.HookInput, wait time.Duration) {
	now := a.clock()
	store := feed.Store{Dir: config.FeedDir(), LockWait: wait}
	switch event {
	case "PostToolUse", "PostToolUseFailure":
		// every tool call comes through here: note it without the lock, and
		// look before taking it
		_ = store.TouchActivity(rec.Handle, now)
		if st, ok := store.Session(rec.Handle); ok && st.State != feed.StateNeedsYou {
			return
		}
	case "Notification":
		// a permission prompt is announced about 6 seconds after it shows
		in.RecentActivity = now.Sub(store.Activity(rec.Handle)) < 6*time.Second
	}
	err := store.UpdateSession(rec.Handle, rec.Account, now, func(s *feed.SessionState) bool {
		return feed.ApplyHook(s, event, in, now)
	})
	if err != nil {
		hookLog(now, event, "%s: update the feed: %v", rec.Handle, err)
	}
	if err := noteClaudeSession(reg, rec, in.SessionID, now); err != nil {
		hookLog(now, event, "%s: record the claude session id: %v", rec.Handle, err)
	}
	if event == "SessionStart" {
		pruneFeed(store, reg, now)
	}
}

// pruneFeed drops the feed files of sessions whose wrapper is gone.
func pruneFeed(store feed.Store, reg registry.Registry, now time.Time) {
	entries, _, err := reg.Scan()
	if err != nil {
		return
	}
	live := map[string]bool{}
	for _, e := range entries {
		if e.Alive && e.Handle != "" {
			live[e.Handle] = true
		}
	}
	_, _ = store.Prune(now, func(h string) bool { return live[h] })
}

// noteClaudeSession records a claude session id that is new to the session, in
// the registry and in the log that usage is attributed by. a /clear or /resume
// gives the same clalt session a new id, and a moved conversation shows up
// under another account with the same one.
func noteClaudeSession(reg registry.Registry, rec registry.Record, id string, now time.Time) error {
	if id == "" || id == rec.ClaudeSessionID {
		return nil
	}
	// the log first: it is what usage is attributed by, and the registry is
	// only a cache of the latest id
	err := usage.AppendSessionAccount(config.SessionAccountsPath(), usage.SessionAccount{Ts: now, SessionID: id, Account: rec.Account})
	if _, _, uerr := reg.Update(rec.Handle, func(r *registry.Record) { r.ClaudeSessionID = id }); uerr != nil && err == nil {
		err = uerr
	}
	return err
}
