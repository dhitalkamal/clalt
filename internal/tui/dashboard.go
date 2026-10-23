package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhitalkamal/clalt/internal/usage"
)

// AccountView is everything the dashboard renders for one account. cli fills it.
type AccountView struct {
	Name  string // the label the account is shown and typed as
	Plan  string
	Email string

	// the sign-in as the state file has it, not asked from claude
	SignedIn bool
	LoginDue time.Time // when the sign-in runs out, zero when signed out

	Five  usage.WindowStat // local 5h window
	Seven usage.WindowStat // local 7d window
	Stats usage.Stats      // full lifetime aggregation
	Daily []usage.DayBucket

	DailyBudget float64 // USD spend cap (0 = unset), the one the detail pane shows
}

// loginSoon is how far ahead an account that has to sign in again is flagged.
const loginSoon = 3 * 24 * time.Hour

// needsLogin is true for an account that is signed out, or has to sign in again
// within a few days.
func (a AccountView) needsLogin() bool {
	return !a.SignedIn || (!a.LoginDue.IsZero() && time.Until(a.LoginDue) < loginSoon)
}

// SessionInfo is one live `clalt run` session shown in the sessions panel.
type SessionInfo struct {
	Account string
	PID     int
	Started time.Time
	Cwd     string
}

// Data is the whole dashboard payload.
type Data struct {
	Version  string
	Accounts []AccountView
	Sessions []SessionInfo

	// Warning is what could not be loaded, such as the usage or the running
	// sessions. it is shown in the header for as long as it holds, so that a
	// missing number does not read as zero. the text is already clean.
	Warning string
}

// Action is a request that needs the real terminal/browser: the caller performs
// it after the alt-screen is torn down, then re-opens the dashboard.
type Action struct {
	Kind    string // "", "run", "login"
	Account string
}

// Controller is the seam between the tui and the cli: the tui defines it, cli.App
// implements it. In-place management actions call these methods and then reload
// via Data; teardown actions (run/login) are returned as an Action instead.
type Controller interface {
	Data() (Data, error)
	Rename(old, new string) error
	SetBudget(name string, daily, weekly float64) error
	Logout(name string) error
	Remove(name string) error
	KillSession(pid int) error
}

// estimate renders a 0..1 fraction as a whole-percent label with a tilde in
// front, because every percent on this dashboard is an estimate.
func estimate(frac float64) string { return "~" + itoa(int(frac*100+0.5)) + "%" }

// fiveFrac is the 5h utilization, and an estimate: the tokens of the last five
// hours that the transcripts hold for the account, over a fixed 2,000,000. it is
// not a share of the plan's limit, which nothing local knows, and it leaves out
// usage that is not attributed to an account.
func fiveFrac(a *AccountView) float64 {
	const ref = 2_000_000.0
	f := float64(a.Five.Tokens) / ref
	if f > 1 {
		f = 1
	}
	return f
}

// RunDashboard shows the interactive multi-panel dashboard and returns the action
// the user chose (performed by the caller after the alt-screen is gone). ctrl
// drives data refresh and in-place management.
func RunDashboard(ctrl Controller, d Data) (Action, error) {
	if len(d.Accounts) == 0 {
		return Action{}, nil
	}
	m := dashModel{data: d, ctrl: ctrl, focus: focusAccounts}
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return Action{}, err
	}
	return final.(dashModel).action, nil
}

// Render produces a single static frame at the given size, for non-TTY output and
// snapshot verification. No controller: no refresh, focus on accounts.
func Render(d Data, width, height int) string {
	m := dashModel{data: d, width: width, height: height, focus: focusAccounts}
	return m.View()
}
