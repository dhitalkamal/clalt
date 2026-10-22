package cli

import (
	"runtime"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/profile"
	"github.com/dhitalkamal/clalt/internal/scheduler"
	"github.com/dhitalkamal/clalt/internal/tui"
	"github.com/dhitalkamal/clalt/internal/usage"
	"github.com/dhitalkamal/clalt/internal/version"
)

// dailyTrendDays is how much history the daily/activity charts cover.
const dailyTrendDays = 45

// buildDashboardData assembles the full tui.Data payload: per-account windows,
// lifetime aggregation, and daily series, with live windows when wantLive.
func (a *App) buildDashboardData(wantLive bool) (tui.Data, error) {
	byAccount, err := a.recordsByAccount()
	if err != nil {
		return tui.Data{}, err
	}
	accts, err := a.Store.List()
	if err != nil {
		return tui.Data{}, err
	}
	now := time.Now()

	data := tui.Data{Version: version.Info().Version}
	nearExpiry := 0
	for i, ac := range accts {
		recs := byAccount[ac.Name]
		view := tui.AccountView{
			Name:         ac.Name,
			Plan:         ac.Meta.Plan,
			Email:        ac.Meta.Email,
			Active:       ac.Active,
			Five:         usage.Window(recs, now, 5*time.Hour),
			Seven:        usage.Window(recs, now, 7*24*time.Hour),
			Stats:        usage.Aggregate(recs, now),
			Daily:        usage.DailyTrend(recs, dailyTrendDays, now),
			DailyBudget:  ac.Meta.DailyBudget,
			WeeklyBudget: ac.Meta.WeeklyBudget,
		}
		if c, _, gerr := a.Store.Get(ac.Name); gerr == nil {
			view.ExpiresAt = c.ExpiresAtTime()
			view.NearExpiry = c.RefreshToken != "" && c.IsExpired(defKeepBuffer)
			if view.NearExpiry {
				nearExpiry++
			}
			if wantLive {
				view.Live, _ = a.liveUsage(ac.Name, c)
			}
		}
		if ac.Active {
			data.Selected = i
		}
		data.Accounts = append(data.Accounts, view)
	}
	data.Keepalive = keepaliveStatus(len(accts), nearExpiry)

	if sessions, serr := profile.ListSessions(config.SessionsDir()); serr == nil {
		for _, s := range sessions {
			data.Sessions = append(data.Sessions, tui.SessionInfo{
				Account: s.Account, PID: s.PID, Started: s.StartedAt, Cwd: s.Cwd,
			})
		}
	}
	return data, nil
}

// keepaliveStatus reports whether the background refresh job is installed on this
// host, plus the token-freshness summary for the keepalive panel. Backend errors
// are treated as "not installed" so the dashboard never fails to open.
func keepaliveStatus(accounts, nearExpiry int) tui.KeepaliveStatus {
	st := tui.KeepaliveStatus{Accounts: accounts, NearExpiry: nearExpiry}
	sched, err := scheduler.For(runtime.GOOS)
	if err != nil {
		return st
	}
	st.Backend = sched.Name()
	if installed, serr := sched.Status(keepaliveLabel); serr == nil {
		st.Installed = installed
	}
	return st
}
