package office

import (
	"runtime"
	"time"

	"github.com/dhitalkamal/clalt/internal/copilot"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
)

// CopilotSource lists the open Copilot CLI sessions it finds on disk.
type CopilotSource interface {
	Sessions() ([]copilot.Session, error)
}

// Real is the machine's own registry, feed and accounts. Now is time.Now when
// nil.
type Real struct {
	Registry registry.Registry
	Feed     feed.Store
	Accounts AccountSource
	Team     TeamSource
	// Copilot reads Copilot CLI's own session files, nil means none.
	Copilot CopilotSource
	// Projects are the folders that hold the project folders of transcripts.
	Projects []string
	Now      func() time.Time
}

// extFresh is how recently a plain claude session must have reported to be
// shown: it has no wrapper whose death would show, only the feed.
const extFresh = 6 * time.Hour

// RealSources wires r into Sources.
func RealSources(r Real) Sources {
	if r.Now == nil {
		r.Now = time.Now
	}
	return Sources{Accounts: r.Accounts, Sessions: r, Limits: r, Subagents: r, Doing: r, Team: r.Team, Now: r.Now}
}

// Sessions lists the sessions whose wrapper is alive, with what the feed says
// they are doing (starting when it has not heard of them), and the plain
// claude and Copilot sessions the feed knows. it only reads.
func (r Real) Sessions() ([]Session, error) {
	entries, _, err := r.Registry.Scan()
	if err != nil {
		return nil, err
	}
	var out []Session
	for _, e := range entries {
		if !e.Alive {
			continue
		}
		s := Session{Handle: e.Handle, Account: e.Account, Cwd: e.Cwd, State: string(feed.StateStarting), StartedAt: e.StartedAt, Can: actionsFor(e.Record, runtime.GOOS)}
		if st, ok := r.Feed.Session(e.Handle); ok {
			s.State, s.Reason, s.Model = string(st.State), st.Reason, st.Model
			if st.Cwd != "" {
				s.Cwd = st.Cwd
			}
		}
		out = append(out, s)
	}
	all, err := r.Feed.Sessions()
	if err != nil {
		return out, nil
	}
	fed := map[string]time.Time{}
	for _, st := range all {
		cp := feed.IsCopilot(st.Handle)
		if !(feed.IsExt(st.Handle) || cp) || st.State == feed.StateEnded || r.Now().Sub(st.UpdatedAt) > extFresh {
			continue
		}
		s := Session{
			Handle: st.Handle, Account: st.Account, Tool: ToolClaude, Cwd: st.Cwd, Model: st.Model, State: string(st.State),
			Reason: st.Reason, StartedAt: st.Since,
		}
		if cp {
			s.Account, s.Tool = CopilotAccount, ToolCopilot
			fed[st.Handle] = st.UpdatedAt
		}
		out = append(out, s)
	}
	return r.withCopilotFiles(out, fed), nil
}

// withCopilotFiles adds the sessions found in Copilot's own folder. when the
// feed has the same handle too, the fresher of the two is listed, once. fed
// maps the Copilot handles already in out to their feed update time.
func (r Real) withCopilotFiles(out []Session, fed map[string]time.Time) []Session {
	if r.Copilot == nil {
		return out
	}
	found, err := r.Copilot.Sessions()
	if err != nil {
		return out
	}
	for _, c := range found {
		s := Session{
			Handle: c.Handle, Account: CopilotAccount, Tool: ToolCopilot, Cwd: c.Cwd, Model: c.Model,
			State: string(c.State), StartedAt: c.StartedAt,
		}
		at, dup := fed[c.Handle]
		if !dup {
			out = append(out, s)
			continue
		}
		if !c.UpdatedAt.After(at) {
			continue
		}
		for i := range out {
			if out[i].Handle == c.Handle {
				out[i] = s
			}
		}
	}
	return out
}

// Limits reads the feed's limits of the account.
func (r Real) Limits(accountID string) (Limits, bool) {
	l, ok := r.Feed.Limits(accountID)
	if !ok || (l.FiveHour == nil && l.SevenDay == nil) {
		return Limits{}, false
	}
	return Limits{FiveHour: win(l.FiveHour), SevenDay: win(l.SevenDay), ReadAt: l.ObservedAt}, true
}

func win(w *feed.Window) *Window {
	if w == nil {
		return nil
	}
	return &Window{UsedPct: w.UsedPct, ResetsAt: w.ResetsAt}
}

// Subagents lists the subagents the feed recorded in the session's state file.
// no file, or no list in it, is no subagents.
func (r Real) Subagents(handle string) []Subagent {
	st, ok := r.Feed.Session(handle)
	if !ok {
		return nil
	}
	out := make([]Subagent, len(st.Subagents))
	for i, sub := range st.Subagents {
		out[i] = Subagent{ID: sub.ID, Type: sub.Type, Label: sub.Label, State: sub.State, StartedAt: sub.StartedAt, EndedAt: sub.EndedAt}
	}
	return out
}
