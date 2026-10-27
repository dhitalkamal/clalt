package cli

import (
	"strconv"
	"strings"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/registry"
	"github.com/dhitalkamal/clalt/internal/terminal"
	"github.com/dhitalkamal/clalt/internal/usage"
)

// registrySessions lists the live records of the session registry as the
// sessions jump and resume read. records of dead processes are removed by the
// registry on the way, as everywhere else.
func registrySessions() ([]terminal.Session, error) {
	recs, err := (registry.Registry{Dir: config.SessionsDir()}).List()
	if err != nil {
		return nil, err
	}
	out := make([]terminal.Session, 0, len(recs))
	for _, r := range recs {
		out = append(out, sessionOf(r))
	}
	return out, nil
}

// sessionOf maps a registry record to a terminal.Session. a kitty window id
// that is not a number stays 0, which focus refuses.
func sessionOf(r registry.Record) terminal.Session {
	win, _ := strconv.Atoi(r.Terminal.KittyWindowID)
	return terminal.Session{
		Handle:          r.Handle,
		Account:         r.Account,
		ClaudeSessionID: r.ClaudeSessionID,
		Cwd:             r.Cwd,
		StartedAt:       r.StartedAt,
		Location: terminal.Location{
			Kind:          terminal.Kind(r.Terminal.Kind),
			KittyWindow:   win,
			KittyListenOn: r.Terminal.KittyListenOn,
			TmuxSocket:    r.Terminal.TmuxSocket,
			TmuxPane:      r.Terminal.TmuxPane,
			IDE:           r.Terminal.IDE,
		},
	}
}

// attributedAccount is the account that last ran a claude session, from the
// log the hooks keep. the latest line for the id wins, whatever the file order.
// an unreadable log counts as not known.
func attributedAccount(claudeSessionID string) (string, bool) {
	lines, err := usage.ReadSessionAccounts(config.SessionAccountsPath())
	if err != nil {
		return "", false
	}
	var best usage.SessionAccount
	found := false
	for _, l := range lines {
		if !strings.EqualFold(l.SessionID, claudeSessionID) {
			continue
		}
		if !found || !l.Ts.Before(best.Ts) {
			best, found = l, true
		}
	}
	return best.Account, found
}
