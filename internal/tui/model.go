package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// focusable panels, cycled by tab / shift+tab.
const (
	focusAccounts = iota
	focusSessions
	focusKeepalive
	focusCount
)

// refreshInterval is how often local data is re-scanned. Live usage still goes
// through its own cache/TTL, so this does not hammer the endpoint.
const refreshInterval = 2 * time.Second

type dashModel struct {
	data Data
	ctrl Controller
	live bool

	focus   int
	sel     int // selected account index within the visible (filtered) list
	sessSel int // selected session index

	width  int
	height int

	action  Action
	flash   string       // transient status line
	mode    string       // "", "filter", "palette"
	query   string       // filter / palette query text
	palSel  int          // selected row in the palette
	modal   inputModal   // active text-input modal (rename/budget/login/add)
	confirm confirmState // active y/N confirmation (remove/logout/kill)

	showHelp   bool
	refreshing bool // a background data reload is in flight
	spin       int  // spinner frame
}

// tickMsg fires on the refresh interval.
type tickMsg struct{}

// dataMsg carries a freshly reloaded payload from the controller.
type dataMsg struct {
	data Data
	err  error
}

// actionDoneMsg carries the result of an in-place management action.
type actionDoneMsg struct{ flash string }

func (m dashModel) Init() tea.Cmd {
	if m.ctrl == nil {
		return nil
	}
	return tea.Batch(tickCmd(), refreshCmd(m.ctrl, m.live))
}

// tickCmd schedules the next refresh tick.
func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

// refreshCmd reloads data off the main loop so a slow scan or live fetch never
// freezes the UI.
func refreshCmd(ctrl Controller, live bool) tea.Cmd {
	if ctrl == nil {
		return nil
	}
	return func() tea.Msg {
		d, err := ctrl.Data(live)
		return dataMsg{data: d, err: err}
	}
}

func (m dashModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		m.refreshing = true
		m.spin = (m.spin + 1) % len(spinFrames)
		return m, tea.Batch(refreshCmd(m.ctrl, m.live), tickCmd())
	case dataMsg:
		m.refreshing = false
		if msg.err == nil {
			m.mergeData(msg.data)
		}
		return m, nil
	case actionDoneMsg:
		m.flash = msg.flash
		return m, refreshCmd(m.ctrl, m.live)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// mergeData swaps in reloaded data while keeping selections in range and the
// current filter query applied.
func (m *dashModel) mergeData(d Data) {
	m.data = d
	if n := len(m.visibleAccounts()); m.sel >= n {
		m.sel = maxInt(0, n-1)
	}
	if n := len(m.data.Sessions); m.sessSel >= n {
		m.sessSel = maxInt(0, n-1)
	}
}

// active returns the account currently selected in the accounts panel (the one
// the detail pane renders). It respects the active filter.
func (m dashModel) active() *AccountView {
	vis := m.visibleAccounts()
	if m.sel < 0 || m.sel >= len(vis) {
		return nil
	}
	return &vis[m.sel]
}

func (m dashModel) viewWidth() int {
	if m.width <= 0 {
		return 120
	}
	return m.width
}

func (m dashModel) viewHeight() int {
	if m.height <= 0 {
		return 40
	}
	return m.height
}
