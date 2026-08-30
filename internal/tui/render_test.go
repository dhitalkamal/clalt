package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dhitalkamal/clalt/internal/usage"
)

func sampleData() Data {
	now := time.Date(2026, 8, 24, 22, 0, 0, 0, time.UTC)
	daily := make([]usage.DayBucket, 45)
	for i := range daily {
		day := now.AddDate(0, 0, -(44 - i))
		// a wavy series so the chart is visibly non-flat
		tok := int64(200_000 + (i%7)*120_000 + (i%3)*90_000)
		daily[i] = usage.DayBucket{Day: day, Tokens: tok, Messages: int(tok / 300), Cost: float64(tok) / 1000}
	}
	main := AccountView{
		Name: "main", Plan: "team", Active: true,
		ExpiresAt: now.Add(6 * time.Hour),
		Five:      usage.WindowStat{Tokens: 340_000, Messages: 346},
		Seven:     usage.WindowStat{Tokens: 5_200_000, Messages: 8100},
		Daily:     daily,
		Stats: usage.Stats{
			Billable: 135_000_000, Input: 5_240, Output: 998_000,
			CacheWrite: 4_000_000, CacheRead: 131_000_000,
			Cost: 17_742.11, Messages: 181_000, Sessions: 229,
			CacheHitRate: 0.97,
			FirstDay:     now.AddDate(0, 0, -38), LastDay: now,
			Today: usage.PeriodStat{Billable: 1_003_000, Input: 5_240, Output: 998_000, Cost: 116.62, Messages: 3337},
			Models: []usage.ModelUsage{
				{Model: "claude-opus-4-8", Billable: 68_700_000, Cost: 12_158.43, Messages: 90000},
				{Model: "claude-sonnet-5", Billable: 66_500_000, Cost: 5_580.81, Messages: 89000},
				{Model: "claude-haiku-4-5", Billable: 127_000, Cost: 2.86, Messages: 2000},
			},
			Projects: []usage.ProjectUsage{
				{Project: "ExampleATS", Billable: 60_000_000, Messages: 90000},
				{Project: "subagents", Billable: 12_000_000, Messages: 20000},
			},
		},
	}
	// give it an hour-of-day shape
	for h := 0; h < 24; h++ {
		main.Stats.HourOfDay[h] = (h*7 + 3) % 40
	}
	main.Stats.HourOfDay[1] = 95
	personal := AccountView{
		Name: "personal", Plan: "pro", ExpiresAt: now.Add(50 * time.Hour),
		Five:  usage.WindowStat{Tokens: 90_000, Messages: 40},
		Seven: usage.WindowStat{Tokens: 1_100_000, Messages: 1500},
		Daily: daily,
		Stats: usage.Stats{Billable: 12_000_000, Cost: 900.0, Messages: 20000, Sessions: 30, FirstDay: now.AddDate(0, 0, -10), LastDay: now},
	}
	return Data{
		Version:  version(),
		Accounts: []AccountView{main, personal},
		Selected: 0,
	}
}

// version is a test stand-in for the cli-provided version string.
func version() string { return "v0.1.0" }

func TestRenderOverview(t *testing.T) {
	out := Render(sampleData(), 120, 40)
	for _, want := range []string{"clalt", "ACCOUNTS", "overview", "main", "personal"} {
		if !strings.Contains(out, want) {
			t.Errorf("overview missing %q", want)
		}
	}
}

func TestRenderTokensTab(t *testing.T) {
	d := sampleData()
	m := dashModel{data: d, sel: 0, width: 120, height: 40, tab: 1}
	out := m.View()
	for _, want := range []string{"SUMMARY", "TOP MODELS", "DAILY", "ACTIVITY", "hit", "today", "total"} {
		if !strings.Contains(out, want) {
			t.Errorf("tokens tab missing %q", want)
		}
	}
}

func TestRenderUsageTab(t *testing.T) {
	d := sampleData()
	m := dashModel{data: d, sel: 0, width: 120, height: 40, tab: 2}
	out := m.View()
	for _, want := range []string{"USAGE", "TOP PROJECTS", "ExampleATS"} {
		if !strings.Contains(out, want) {
			t.Errorf("usage tab missing %q", want)
		}
	}
}

func TestDeleteConfirmFlow(t *testing.T) {
	d := sampleData()
	m := dashModel{data: d, sel: 1} // personal (not active)

	// press d -> enters confirmation, footer shows the prompt
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = nm.(dashModel)
	if !m.confirming || m.confirmFor != "personal" {
		t.Fatalf("expected confirming personal, got %v %q", m.confirming, m.confirmFor)
	}
	if !strings.Contains(m.View(), "delete personal ?") {
		t.Error("footer should show the delete confirmation")
	}
	// y -> emits a remove action
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = nm.(dashModel)
	if m.action.Kind != "remove" || m.action.Account != "personal" {
		t.Errorf("expected remove action for personal, got %+v", m.action)
	}
}

func TestDeleteActiveGuarded(t *testing.T) {
	d := sampleData()
	m := dashModel{data: d, sel: 0} // main is active
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = nm.(dashModel)
	if m.confirming {
		t.Error("should not confirm-delete the active account")
	}
	if !strings.Contains(m.View(), "cannot remove the active account") {
		t.Error("expected a guard message for deleting the active account")
	}
}

func TestFilterMode(t *testing.T) {
	m := dashModel{data: sampleData(), sel: 0}
	// open filter, type "per" -> only personal visible
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = nm.(dashModel)
	if m.mode != "filter" {
		t.Fatalf("expected filter mode, got %q", m.mode)
	}
	for _, r := range "per" {
		nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = nm.(dashModel)
	}
	vis := m.visibleAccounts()
	if len(vis) != 1 || vis[0].Name != "personal" {
		t.Errorf("filter 'per' -> %d accounts, want [personal]", len(vis))
	}
}

func TestPaletteMode(t *testing.T) {
	m := dashModel{data: sampleData(), sel: 0}
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	m = nm.(dashModel)
	if m.mode != "palette" {
		t.Fatalf("expected palette mode, got %q", m.mode)
	}
	// first command is "run main"; enter emits it (keychain-free launch)
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(dashModel)
	if m.action.Kind != "run" || m.action.Account != "main" {
		t.Errorf("palette enter -> %+v, want run main", m.action)
	}
}

func TestRenderHelpOverlay(t *testing.T) {
	d := sampleData()
	m := dashModel{data: d, sel: 0, width: 120, height: 40, showHelp: true}
	out := m.View()
	for _, want := range []string{"KEYBINDINGS", "run selected account", "delete the selected account"} {
		if !strings.Contains(out, want) {
			t.Errorf("help overlay missing %q", want)
		}
	}
}
