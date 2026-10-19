package dashboard

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
)

type pane int

const (
	paneAccounts pane = iota
	paneDetail
)

type flash struct {
	text string
	err  bool
	id   int // keeps an old timer from clearing a newer flash
}

type model struct {
	ctx context.Context
	src Source
	act Actions
	opt Options
	th  theme

	snap      Snapshot
	have      bool      // a snapshot has arrived
	loading   bool      // a load is in flight
	dirty     bool      // something changed while loading, load again after
	loadErr   error     // why the last load failed, nil when it worked
	loadedAt  time.Time // when the last good load finished
	loadStart time.Time // when the current or last load began
	nextLoad  time.Time // earliest start of the next refresh

	w, h      int
	focus     pane
	view      viewID
	hist      int    // history tab: days, projects, models
	sel       string // id of the selected account
	listOff   int    // first account row shown when the list is longer than the pane
	rowCur    int    // cursor row in the right pane lists
	sessID    string // the session under the cursor, kept by id because the list changes
	scroll    int    // first visible body line in the right pane
	filter    string
	filtering bool
	help      bool
	helpAt    int // first help line shown when the list is taller than the screen
	flash     flash
	busy      bool // an action is in flight, so another one must wait
	entry     entry
	confirm   confirm
	drill     drill
}

func newModel(ctx context.Context, src Source, act Actions, opt Options) model {
	opt = opt.withDefaults()
	return model{
		ctx: ctx, src: src, act: act, opt: opt,
		th:        theme{p: newPalette(opt.NoColor), g: glyphsFor(opt.ASCII)},
		loading:   true, // Init starts the first load
		loadStart: opt.Now(),
	}
}

// New is a dashboard that loads from src, acts through act and runs until the
// user quits. ctx ends any load still running.
func New(ctx context.Context, src Source, act Actions, opt Options) tea.Model {
	return newModel(ctx, src, act, opt)
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.loadCmd(), m.tickCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m.synced(), nil
	case loadedMsg:
		next, cmd := m.onLoaded(msg)
		return next.synced(), cmd
	case tickMsg:
		return m.onTick()
	case flashExpiredMsg:
		if msg.id == m.flash.id {
			m.flash.text = ""
		}
		return m, nil
	case launchMsg:
		return m.onLaunch(msg)
	case execDoneMsg:
		return m.onExecDone(msg)
	case actionDoneMsg:
		return m.onActionDone(msg)
	case tea.PasteMsg:
		return m.onPaste(msg.Content), nil
	case tea.KeyPressMsg:
		next, cmd := m.onKey(msg)
		return next.synced(), cmd
	}
	return m, nil
}

func (m model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "clalt"
	v.Cursor = m.cursor()
	return v
}
