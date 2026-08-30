package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhitalkamal/clalt/internal/usage"
)

// AccountView is everything the dashboard renders for one account. cli fills it.
type AccountView struct {
	Name      string
	Plan      string
	Email     string
	Active    bool
	ExpiresAt time.Time

	Five  usage.WindowStat // local 5h window
	Seven usage.WindowStat // local 7d window
	Live  *usage.LiveUsage // authoritative windows, nil unless --live
	Stats usage.Stats      // full lifetime aggregation
	Daily []usage.DayBucket

	DailyBudget  float64 // USD spend cap (0 = unset)
	WeeklyBudget float64
	NearExpiry   bool // token expires within the keepalive buffer
}

// SessionInfo is one live `clalt run` session shown in the sessions panel.
type SessionInfo struct {
	Account string
	PID     int
	Started time.Time
	Cwd     string
}

// KeepaliveStatus summarizes the background refresh job for the keepalive panel.
type KeepaliveStatus struct {
	Installed  bool
	Backend    string // launchd / systemd / schtasks
	NearExpiry int    // accounts whose token expires within the buffer
	Accounts   int    // total accounts
}

// Data is the whole dashboard payload.
type Data struct {
	Version   string
	Accounts  []AccountView
	Selected  int
	Sessions  []SessionInfo
	Keepalive KeepaliveStatus
}

// Action is a request that needs the real terminal/browser: the caller performs
// it after the alt-screen is torn down, then re-opens the dashboard.
type Action struct {
	Kind    string // "", "run", "login", "add"
	Account string
}

// Controller is the seam between the tui and the cli: the tui defines it, cli.App
// implements it. In-place management actions call these methods and then reload
// via Data; teardown actions (run/login/add) are returned as an Action instead.
type Controller interface {
	Data(live bool) (Data, error)
	Rename(old, new string) error
	SetBudget(name string, daily, weekly float64) error
	Logout(name string) error
	Remove(name string) error
	Refresh(name string) error
	KillSession(pid int) error
}

// pct renders a 0..1 fraction as a whole-percent label.
func pct(frac float64) string { return itoa(int(frac*100+0.5)) + "%" }

// liveFiveFrac is the 5h utilization: live if present, else a local proxy scaled
// to a nominal reference (there is no local hard cap).
func liveFiveFrac(a *AccountView) float64 {
	if a.Live != nil {
		return a.Live.FiveHour.Utilization
	}
	const ref = 2_000_000.0
	f := float64(a.Five.Tokens) / ref
	if f > 1 {
		f = 1
	}
	return f
}

// RunDashboard shows the interactive multi-panel dashboard and returns the action
// the user chose (performed by the caller after the alt-screen is gone). ctrl
// drives live refresh and in-place management; live gates the usage endpoint.
func RunDashboard(ctrl Controller, d Data, live bool) (Action, error) {
	if len(d.Accounts) == 0 {
		return Action{}, nil
	}
	m := dashModel{data: d, ctrl: ctrl, live: live, sel: d.Selected, focus: focusAccounts}
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return Action{}, err
	}
	return final.(dashModel).action, nil
}

// Render produces a single static frame at the given size, for non-TTY output and
// snapshot verification. No controller: no live refresh, focus on accounts.
func Render(d Data, width, height int) string {
	m := dashModel{data: d, sel: d.Selected, width: width, height: height, focus: focusAccounts}
	return m.View()
}
