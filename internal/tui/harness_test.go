package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhitalkamal/clalt/internal/usage"
)

// test harness: a recording fake controller, representative sample data, and
// helpers to pump keys and drain command chains synchronously.

// fakeController records controller calls so tests can assert the in-place
// management actions delegate correctly. Data returns whatever is set on it.
type fakeController struct {
	data Data

	renamed   [2]string
	budget    [3]string // name, daily, weekly (as raw floats formatted)
	loggedOut string
	removed   string
	refreshed string
	killed    int

	failNext error
}

func (f *fakeController) Data(bool) (Data, error) { return f.data, nil }
func (f *fakeController) Rename(old, n string) error {
	f.renamed = [2]string{old, n}
	return f.failNext
}
func (f *fakeController) SetBudget(name string, daily, weekly float64) error {
	f.budget = [3]string{name, ftoa(daily), ftoa(weekly)}
	return f.failNext
}
func (f *fakeController) Logout(name string) error { f.loggedOut = name; return f.failNext }
func (f *fakeController) Remove(name string) error { f.removed = name; return f.failNext }
func (f *fakeController) Refresh(name string) error {
	f.refreshed = name
	return f.failNext
}
func (f *fakeController) KillSession(pid int) error { f.killed = pid; return f.failNext }

func ftoa(v float64) string {
	if v == float64(int64(v)) {
		return itoa(int(v))
	}
	return "x"
}

func sampleData() Data {
	now := time.Now().UTC()
	daily := make([]usage.DayBucket, 45)
	for i := range daily {
		day := now.AddDate(0, 0, -(44 - i))
		tok := int64(200_000 + (i%7)*120_000 + (i%3)*90_000)
		daily[i] = usage.DayBucket{Day: day, Tokens: tok, Messages: int(tok / 300), Cost: float64(tok) / 1000}
	}
	work := AccountView{
		Name: "work", Plan: "max", Active: true, Email: "work@acme.com",
		ExpiresAt: now.Add(3*time.Hour + 12*time.Minute),
		Five:      usage.WindowStat{Tokens: 340_000, Messages: 346},
		Seven:     usage.WindowStat{Tokens: 5_200_000, Messages: 8100},
		Daily:     daily,
		Live: &usage.LiveUsage{
			FiveHour: usage.LiveWindow{Utilization: 0.62, ResetsAt: now.Add(2*time.Hour + 10*time.Minute)},
			SevenDay: usage.LiveWindow{Utilization: 0.41, ResetsAt: now.AddDate(0, 0, 5)},
		},
		DailyBudget: 10, WeeklyBudget: 50, NearExpiry: true,
		Stats: usage.Stats{
			Billable: 135_000_000, Cost: 17_742.11, Messages: 181_000, Sessions: 229,
			CacheHitRate: 0.97, FirstDay: now.AddDate(0, 0, -38), LastDay: now,
			Today: usage.PeriodStat{Billable: 1_200_000, Cost: 2.14, Messages: 3337},
			Models: []usage.ModelUsage{
				{Model: "claude-opus-4-8", Billable: 68_700_000, Cost: 12_158.43, Messages: 90000},
				{Model: "claude-sonnet-5", Billable: 66_500_000, Cost: 5_580.81, Messages: 89000},
			},
			Projects: []usage.ProjectUsage{
				{Project: "clalt", Billable: 60_000_000, Messages: 90000},
				{Project: "rhr", Billable: 12_000_000, Messages: 20000},
			},
		},
	}
	for h := 0; h < 24; h++ {
		work.Stats.HourOfDay[h] = (h*7 + 3) % 40
	}
	work.Stats.HourOfDay[1] = 95
	personal := AccountView{
		Name: "personal", Plan: "pro", Email: "me@home.dev", ExpiresAt: now.Add(50 * time.Hour),
		Five:  usage.WindowStat{Tokens: 90_000, Messages: 40},
		Seven: usage.WindowStat{Tokens: 1_100_000, Messages: 1500},
		Daily: daily,
		Stats: usage.Stats{Billable: 12_000_000, Cost: 900.0, Messages: 20000, Sessions: 30, FirstDay: now.AddDate(0, 0, -10), LastDay: now},
	}
	casey := AccountView{Name: "casey", Plan: "max", Five: usage.WindowStat{Tokens: 700_000}, Daily: daily}
	jordan := AccountView{Name: "jordan", Plan: "pro", Five: usage.WindowStat{Tokens: 1_900_000}, NearExpiry: true, Daily: daily}
	return Data{
		Version:   "v0.1.0",
		Accounts:  []AccountView{work, personal, casey, jordan},
		Selected:  0,
		Sessions:  []SessionInfo{{Account: "work", PID: 8123, Started: now.Add(-2*time.Hour - 4*time.Minute), Cwd: "/home/k/clalt"}},
		Keepalive: KeepaliveStatus{Installed: true, Backend: "launchd", NearExpiry: 2, Accounts: 4},
	}
}

// newTestModel builds a model wired to a fake controller at a usable size, focus
// on the accounts panel.
func newTestModel(f *fakeController) dashModel {
	f.data = sampleData()
	return dashModel{data: f.data, ctrl: f, width: 120, height: 40, sel: 0, focus: focusAccounts}
}

// key builds a KeyMsg for the given token.
func key(s string) tea.Msg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// drain runs a returned cmd chain synchronously (never a tick), feeding messages
// back into Update so controller-backed actions actually execute during a test.
func drain(m dashModel, cmd tea.Cmd) dashModel {
	for i := 0; cmd != nil && i < 20; i++ {
		msg := cmd()
		if msg == nil {
			break
		}
		if _, ok := msg.(tickMsg); ok {
			break
		}
		var next tea.Cmd
		nm, c := m.Update(msg)
		m, next = nm.(dashModel), c
		cmd = next
	}
	return m
}

// pump feeds a sequence of key tokens through Update, draining each cmd chain.
func pump(m dashModel, keys ...string) dashModel {
	for _, k := range keys {
		nm, cmd := m.Update(key(k))
		m = nm.(dashModel)
		m = drain(m, cmd)
	}
	return m
}
