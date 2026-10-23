package live

import (
	"context"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
)

// RegistrySessions is Sessions on the session registry and the feed. Probe and
// Signal are how Stop looks at a process and ends it, with the real ones when
// nil.
type RegistrySessions struct {
	Reg    registry.Registry
	Feed   feed.Store
	Now    func() time.Time
	Probe  func(ctx context.Context, pid int) (Proc, error)
	Signal func(pid int) error
}

var _ Sessions = (*RegistrySessions)(nil)

func (r *RegistrySessions) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// Running lists the sessions whose wrapper is alive, oldest first, each with
// what the feed says it is doing. it only reads: a record of a dead session is
// skipped and left for the commands that clean up. a session the feed has not
// heard of yet is starting.
func (r *RegistrySessions) Running(context.Context) ([]Session, error) {
	entries, _, err := r.Reg.Scan()
	if err != nil {
		return nil, err
	}
	var out []Session
	for _, e := range entries {
		if !e.Alive {
			continue
		}
		s := Session{
			Handle: e.Handle, AccountID: e.Account, ClaudeSessionID: e.ClaudeSessionID,
			State: StateStarting, Cwd: e.Cwd, StartedAt: e.StartedAt, UpdatedAt: e.StartedAt,
		}
		if st, ok := r.Feed.Session(e.Handle); ok {
			s.State, s.Reason, s.Model, s.ContextPct, s.UpdatedAt = SessionState(st.State), st.Reason, st.Model, st.ContextPct, st.UpdatedAt
			if st.Cwd != "" {
				s.Cwd = st.Cwd
			}
		}
		s.Branch = gitBranch(s.Cwd)
		out = append(out, s)
	}
	return out, nil
}
