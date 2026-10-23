package cli

import (
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/pick"
	"github.com/dhitalkamal/clalt/internal/registry"
	"github.com/dhitalkamal/clalt/internal/wrapper"
)

// registryAdapter is the session registry as the wrapper sees it. store is the
// feed, which a move has to tell about the new account.
type registryAdapter struct {
	reg   registry.Registry
	store feed.Store
	now   func() time.Time
}

var _ wrapper.Registry = registryAdapter{}

func sessionRegistry() registryAdapter {
	return registryAdapter{
		reg: registry.Registry{Dir: config.SessionsDir()}, store: feed.Store{Dir: config.FeedDir()}, now: time.Now,
	}
}

// List is the live records. a request is written next to them, see Dir.
func (r registryAdapter) List() ([]wrapper.Record, error) {
	recs, err := r.reg.List()
	if err != nil {
		return nil, err
	}
	out := make([]wrapper.Record, len(recs))
	for i, rec := range recs {
		out[i] = wrapper.Record{
			Handle: rec.Handle, Account: rec.Account, Folder: rec.Folder, WrapperPID: rec.WrapperPID,
			ClaudePID: rec.PID, ClaudeSessionID: rec.ClaudeSessionID, Cwd: rec.Cwd,
			Terminal: rec.Terminal.Kind, StartedAt: rec.StartedAt,
		}
	}
	return out, nil
}

// Dir is the folder of the records.
func (r registryAdapter) Dir() string { return r.reg.Dir }

// Update writes what changed after claude was started. a new account also puts
// the feed back to "starting" under it: the old claude left the session idle,
// and the new one has not said anything yet, so that idle must not be read as
// its pause.
func (r registryAdapter) Update(handle string, c wrapper.Change) error {
	_, _, err := r.reg.Update(handle, func(rec *registry.Record) {
		if c.ClaudePID != 0 {
			rec.PID = c.ClaudePID
		}
		if c.Account != "" {
			rec.Account = c.Account
		}
		if c.Folder != "" {
			rec.Folder = c.Folder
		}
	})
	if err != nil || c.Account == "" {
		return err
	}
	now := r.now()
	return r.store.UpdateSession(handle, c.Account, now, func(s *feed.SessionState) bool {
		s.Account, s.State, s.Since, s.Reason, s.LastError = c.Account, feed.StateStarting, now, "", ""
		return true
	})
}

// feedAdapter is the feed as the wrapper sees it.
type feedAdapter struct{ store feed.Store }

var _ wrapper.Feed = feedAdapter{}

func realFeed() feedAdapter { return feedAdapter{store: feed.Store{Dir: config.FeedDir()}} }

var feedStates = map[feed.State]wrapper.State{
	feed.StateStarting: wrapper.Starting,
	feed.StateBusy:     wrapper.Busy,
	feed.StateIdle:     wrapper.Idle,
	feed.StateNeedsYou: wrapper.NeedsYou,
	feed.StateEnded:    wrapper.Ended,
}

// Session is what the hooks last said about the session. a stop that failed
// with rate_limit is the account's limit being hit.
func (f feedAdapter) Session(handle string) (wrapper.SessionInfo, bool) {
	st, ok := f.store.Session(handle)
	if !ok {
		return wrapper.SessionInfo{}, false
	}
	return wrapper.SessionInfo{
		State:    feedStates[st.State],
		LimitHit: st.State == feed.StateIdle && st.LastError == "rate_limit",
		Cwd:      st.Cwd, UpdatedAt: st.UpdatedAt,
	}, true
}

// Limits are the windows the feed last saw for an account.
func (f feedAdapter) Limits(accountID string) (pick.Limits, bool) {
	l, ok := f.store.Limits(accountID)
	if !ok {
		return pick.Limits{}, false
	}
	return pick.Limits{FiveHour: pickWindow(l.FiveHour), SevenDay: pickWindow(l.SevenDay), ObservedAt: l.ObservedAt}, true
}

func pickWindow(w *feed.Window) pick.Window {
	if w == nil {
		return pick.Window{}
	}
	return pick.Window{Used: w.UsedPct, ResetsAt: w.ResetsAt, Known: true}
}
