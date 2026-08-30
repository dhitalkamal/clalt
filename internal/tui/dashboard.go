package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhitalkamal/clalt/internal/usage"
)

// AccountView is everything the dashboard renders for one account. cli fills it.
type AccountView struct {
	Name      string
	Plan      string
	Active    bool
	ExpiresAt time.Time

	Five  usage.WindowStat // local 5h window
	Seven usage.WindowStat // local 7d window
	Live  *usage.LiveUsage // authoritative windows, nil unless --live
	Stats usage.Stats      // full lifetime aggregation
	Daily []usage.DayBucket

	DailyBudget  float64 // USD spend cap (0 = unset)
	WeeklyBudget float64
}

// SessionInfo is one live `clalt run` session shown on the sessions tab.
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
	Selected int
	Sessions []SessionInfo
}

// Action is what the user asked for on exit, performed by the caller once the
// alt-screen has been torn down.
type Action struct {
	Kind    string // "", "run", "remove"
	Account string
}

// tabs shown across the top.
var tabs = []string{"overview", "tokens", "usage", "sessions"}

type dashModel struct {
	data       Data
	tab        int
	sel        int // selected account index
	width      int
	height     int
	showHelp   bool
	action     Action
	confirming bool   // awaiting y/n on a delete
	confirmFor string // account pending deletion
	flash      string // transient status message
	mode       string // "", "filter", "palette"
	query      string // filter / palette query text
	palSel     int    // selected row in the palette
}

func (m dashModel) Init() tea.Cmd { return nil }

func (m dashModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if m.showHelp {
			m.showHelp = false // any key closes help
			return m, nil
		}
		if m.mode == "filter" {
			return m.updateFilter(msg)
		}
		if m.mode == "palette" {
			return m.updatePalette(msg)
		}
		if m.confirming {
			if k := msg.String(); k == "y" || k == "Y" {
				m.action = Action{Kind: "remove", Account: m.confirmFor}
				return m, tea.Quit
			}
			m.confirming = false
			return m, nil
		}
		m.flash = "" // clear any transient message on the next keypress
		vis := m.visibleAccounts()
		switch msg.String() {
		case "left", "h":
			if m.tab > 0 {
				m.tab--
			}
		case "right", "l", "tab":
			if m.tab < len(tabs)-1 {
				m.tab++
			}
		case "up", "k":
			if m.sel > 0 {
				m.sel--
			}
		case "down", "j":
			if m.sel < len(vis)-1 {
				m.sel++
			}
		case "enter", "r":
			// launch a keychain-free `clalt run` session for the selected account
			if a := m.selectedAccount(); a != nil {
				m.action = Action{Kind: "run", Account: a.Name}
				return m, tea.Quit
			}
		case "d", "x":
			if a := m.selectedAccount(); a != nil {
				if a.Active {
					m.flash = "cannot remove the active account"
				} else {
					m.confirming, m.confirmFor = true, a.Name
				}
			}
		case "/":
			m.mode, m.query, m.tab = "filter", "", 0
		case ":":
			m.mode, m.query, m.palSel = "palette", "", 0
		case "?":
			m.showHelp = true
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m dashModel) View() string {
	if len(m.data.Accounts) == 0 {
		return "no accounts to report on\n"
	}
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	b.WriteString(m.tabBar())
	b.WriteString("\n\n")

	if m.showHelp {
		b.WriteString(m.helpView())
	} else if m.mode == "palette" {
		b.WriteString(m.paletteView())
	} else {
		switch tabs[m.tab] {
		case "overview":
			b.WriteString(m.viewOverview())
		case "tokens":
			b.WriteString(m.viewTokens())
		case "usage":
			b.WriteString(m.viewUsage())
		case "sessions":
			b.WriteString(m.viewSessions())
		}
	}

	b.WriteString("\n")
	b.WriteString(m.footer())
	return b.String()
}

// active returns the currently selected account view.
func (m dashModel) active() *AccountView {
	if m.sel < 0 || m.sel >= len(m.data.Accounts) {
		return nil
	}
	return &m.data.Accounts[m.sel]
}

func (m dashModel) viewWidth() int {
	if m.width <= 0 {
		return 96 // sensible default before the first WindowSizeMsg
	}
	return m.width
}

// liveFiveFrac is the 5h utilization for the header meter: live if present, else
// a local proxy scaled to a nominal reference (there is no local hard cap).
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

func pct(frac float64) string {
	return itoa(int(frac*100+0.5)) + "%"
}

// RunDashboard shows the interactive dashboard and returns the action the user
// chose (performed by the caller after the alt-screen is gone).
func RunDashboard(d Data) (Action, error) {
	if len(d.Accounts) == 0 {
		return Action{}, nil
	}
	m := dashModel{data: d, sel: d.Selected}
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return Action{}, err
	}
	return final.(dashModel).action, nil
}

// Render produces a single static frame at the given size, for non-TTY output
// and snapshot verification.
func Render(d Data, width, height int) string {
	m := dashModel{data: d, sel: d.Selected, width: width, height: height}
	return m.View()
}
