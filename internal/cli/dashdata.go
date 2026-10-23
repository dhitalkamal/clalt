package cli

import (
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/profile"
	"github.com/dhitalkamal/clalt/internal/tui"
	"github.com/dhitalkamal/clalt/internal/usage"
	"github.com/dhitalkamal/clalt/internal/version"
)

// dailyTrendDays is how much history the daily/activity charts cover.
const dailyTrendDays = 45

// buildDashboardData assembles the tui.Data payload: the visible accounts by
// label, each with the sign-in the state file records (the dashboard does not
// ask claude) and its local usage, and the live sessions.
func (a *App) buildDashboardData() (tui.Data, error) {
	accts, err := a.visibleAccounts()
	if err != nil {
		return tui.Data{}, cleanErr(err)
	}
	labels, err := a.labelsByID()
	if err != nil {
		return tui.Data{}, cleanErr(err)
	}
	var warnings []string
	// usage that cannot be read is a warning on a dashboard that still lists the
	// accounts, and is not zero
	byAccount, err := a.recordsByAccount()
	if err != nil {
		warnings = append(warnings, "usage could not be read: "+clean(err.Error()))
	}
	now := a.clock()

	data := tui.Data{Version: version.Info().Version}
	for _, acc := range accts {
		recs := byAccount[acc.Name]
		view := tui.AccountView{
			Name:        shown(acc),
			Plan:        clean(acc.Meta.Plan),
			Email:       clean(acc.Meta.Email),
			SignedIn:    !acc.Meta.SignedInAt.IsZero(),
			Five:        usage.Window(recs, now, 5*time.Hour),
			Seven:       usage.Window(recs, now, 7*24*time.Hour),
			Stats:       cleanStats(usage.Aggregate(recs, now)),
			Daily:       usage.DailyTrend(recs, dailyTrendDays, now),
			DailyBudget: acc.Meta.DailyBudget,
		}
		if view.SignedIn {
			view.LoginDue = loginDue(acc.Meta.SignedInAt)
		}
		data.Accounts = append(data.Accounts, view)
	}

	sessions, err := profile.ListSessions(config.SessionsDir())
	if err != nil {
		// an empty list that is really a folder that cannot be read is not "no sessions"
		warnings = append(warnings, "the running sessions could not be read: "+clean(err.Error()))
	}
	for _, s := range sessions {
		name, known := labels[s.Account]
		if !known {
			name = s.Account
		}
		data.Sessions = append(data.Sessions, tui.SessionInfo{
			Account: clean(name), PID: s.PID, Started: s.StartedAt, Cwd: clean(s.Cwd),
		})
	}
	data.Warning = strings.Join(warnings, "; ")
	return data, nil
}

// cleanStats cleans the model ids and project names that came out of the
// transcripts, which anything can write to and the dashboard draws.
func cleanStats(s usage.Stats) usage.Stats {
	for i := range s.Models {
		s.Models[i].Model = clean(s.Models[i].Model)
	}
	for i := range s.Projects {
		s.Projects[i].Project = clean(s.Projects[i].Project)
	}
	return s
}
