package live

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/claudecli"
	"github.com/dhitalkamal/clalt/internal/dashboard"
)

const notSignedIn = claudecli.ReasonNotSignedIn

// sessionsByAccount asks the registry and gives the running sessions by
// account id, oldest first. a registry that does not exist yet is a warning of
// its own, since "no sessions" would be a claim.
func (a *Adapter) sessionsByAccount(ctx context.Context, now time.Time, led *ledger) (map[string][]dashboard.Session, []string) {
	var warns []string
	if _, none := a.cfg.Sessions.(NoSessions); none {
		warns = append(warns, "running sessions are not shown yet")
	}
	list, err := a.cfg.Sessions.Running(ctx)
	if err != nil {
		return nil, append(warns, "running sessions could not be read: "+err.Error())
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].StartedAt.Before(list[j].StartedAt) })
	out := map[string][]dashboard.Session{}
	for _, s := range list {
		if s.State == StateEnded {
			continue
		}
		used, size := contextTokens(s)
		out[s.AccountID] = append(out[s.AccountID], dashboard.Session{
			ID: s.Handle, State: sessionState(s.State), Folder: shorten(s.Cwd, a.cfg.Home), Model: modelName(s.Model),
			Branch: s.Branch, ContextUsed: used, ContextSize: size,
			StartedAt: s.StartedAt, Cost: led.sessionCost[s.ClaudeSessionID],
		})
	}
	return out, warns
}

func sessionState(s SessionState) dashboard.SessionState {
	switch s {
	case StateBusy:
		return dashboard.StateWorking
	case StateNeedsYou:
		return dashboard.StateWaiting
	}
	return dashboard.StateIdle
}

// contextTokens turns the statusline's percent into tokens. the window size is
// not reported: a model id marked [1m] has a million, anything else 200k, so
// the figure is an estimate and the percent is what to trust.
func contextTokens(s Session) (used, size int64) {
	if s.ContextPct == nil {
		return 0, 0
	}
	size = 200_000
	if strings.Contains(s.Model, "[1m]") {
		size = 1_000_000
	}
	return int64(*s.ContextPct / 100 * float64(size)), size
}
