package mcp

import (
	"testing"

	"github.com/dhitalkamal/clalt/internal/office"
)

// withCopilot is fake() plus the Copilot team and one of its sessions. the
// team carries a limit on purpose: it must still be ignored.
func withCopilot() Backend {
	b := fake()
	snap, _ := b.Snapshot()
	snap.Accounts = append(snap.Accounts, office.AccountOut{
		ID: "~copilot", Label: "Copilot", Tool: "copilot", Limits: office.LimitsOut{FiveHour: win(1, 4*3600e9)},
	})
	snap.Sessions = append(snap.Sessions, office.SessionOut{
		Handle: "cp-abc", Account: "~copilot", Tool: "copilot", Project: "site", State: "busy", StartedAt: t0, Subagents: []office.Subagent{},
	})
	b.Snapshot = func() (office.Snapshot, error) { return snap, nil }
	return b
}

func TestLimitsAndBestAccountIgnoreTheCopilotTeam(t *testing.T) {
	b := withCopilot()

	accts := payload(t, call(t, b, "limits", `{}`))["accounts"].([]any)
	if len(accts) != 3 {
		t.Errorf("limits listed %d accounts", len(accts))
	}
	for _, a := range accts {
		if a.(map[string]any)["id"] == "~copilot" {
			t.Error("copilot in limits")
		}
	}
	best := payload(t, call(t, b, "best_account", `{}`))["best"].(map[string]any)
	if best["id"] != "acc-1" {
		t.Errorf("best = %v", best)
	}
}

func TestSessionsSayTheirTool(t *testing.T) {
	list := payload(t, call(t, withCopilot(), "sessions", `{}`))["sessions"].([]any)
	tools := map[string]any{}
	for _, s := range list {
		m := s.(map[string]any)
		tools[m["handle"].(string)] = m["tool"]
	}
	if tools["cp-abc"] != "copilot" || tools["h1"] != "claude" {
		t.Errorf("%v", tools)
	}
}
