package office

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/office/layout"
)

func copilotHandle(t *testing.T, id string) string {
	t.Helper()
	h, ok := feed.CopilotHandle(id)
	if !ok {
		t.Fatal("bad id")
	}
	return h
}

func TestCopilotSessionsAreListedLikeExtOnesInTheCopilotTeam(t *testing.T) {
	f := newFixture(t)
	live := copilotHandle(t, "aaaa-1111")
	f.state(live, "", feed.StateIdle, t0.Add(-time.Minute), func(s *feed.SessionState) { s.Cwd = "/p/site" })
	f.state(copilotHandle(t, "bbbb-2222"), "", feed.StateEnded, t0.Add(-time.Minute), nil)
	f.state(copilotHandle(t, "cccc-3333"), "", feed.StateBusy, t0.Add(-7*time.Hour), nil)

	snap := f.build()

	if len(snap.Sessions) != 1 {
		t.Fatalf("%+v", snap.Sessions)
	}
	s := snap.Sessions[0]
	if s.Handle != live || s.Account != CopilotAccount || s.Tool != ToolCopilot || s.Project != "site" {
		t.Errorf("%+v", s)
	}
	if s.Actions.Move || s.Actions.Focus || s.Actions.Stop || s.Actions.Why != "a Copilot session: clalt only shows it" {
		t.Errorf("actions %+v", s.Actions)
	}
}

func TestTheCopilotTeamComesAfterTheClaudeAccountsAndOnlyWithASession(t *testing.T) {
	f := newFixture(t)
	f.accs = []Account{{ID: "work", Label: "work"}, {ID: "zed", Label: "zed"}}
	f.state(runHandle, "work", feed.StateIdle, t0, nil)
	if got := f.build().Accounts; len(got) != 2 {
		t.Fatalf("no copilot session but %+v", got)
	}

	f.state(copilotHandle(t, "aaaa-1111"), "", feed.StateBusy, t0, nil)
	got := f.build().Accounts

	if len(got) != 3 || got[0].ID != "work" || got[1].ID != "zed" {
		t.Fatalf("%+v", got)
	}
	cp := got[2]
	if cp.ID != CopilotAccount || cp.Label != "Copilot" || cp.Tool != ToolCopilot || cp.Limits.FiveHour != nil || cp.Limits.SevenDay != nil || cp.Limits.ReadAt != nil || cp.Forecast.FiveHourAt != nil {
		t.Errorf("%+v", cp)
	}
	if got[0].Tool != ToolClaude {
		t.Errorf("claude tool %q", got[0].Tool)
	}
}

func TestAnEndedCopilotSessionAloneDoesNotMakeATeam(t *testing.T) {
	f := newFixture(t)
	f.state(copilotHandle(t, "aaaa-1111"), "", feed.StateEnded, t0, nil)
	if got := f.build().Accounts; len(got) != 2 {
		t.Errorf("%+v", got)
	}
}

func TestTheSnapshotSaysTheToolOfEverySession(t *testing.T) {
	f := newFixture(t)
	f.record(runHandle, "work", "/p/x", os.Getpid())
	f.state(copilotHandle(t, "aaaa-1111"), "", feed.StateBusy, t0, nil)

	raw, err := json.Marshal(f.build())
	if err != nil {
		t.Fatal(err)
	}
	var back struct {
		Accounts []map[string]any
		Sessions []map[string]any
	}
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	tools := map[string]string{}
	for _, s := range back.Sessions {
		tools[s["handle"].(string)], _ = s["tool"].(string)
	}
	if tools[runHandle] != "claude" || tools[copilotHandle(t, "aaaa-1111")] != "copilot" {
		t.Errorf("%v in %s", tools, raw)
	}
	for _, a := range back.Accounts {
		if _, ok := a["tool"].(string); !ok {
			t.Errorf("account without tool: %v", a)
		}
	}
	if !strings.Contains(string(raw), `"v":1`) {
		t.Error("version moved")
	}
}

func TestTheCopilotTeamGetsNoForecastEvenIfALimitsSourceAnswersForIt(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	lim := fcLimits{"~copilot": {FiveHour: win5(75, now.Add(2*time.Hour)), ReadAt: now}}
	snap, err := Build(Sources{
		Accounts: fcAccounts{}, Limits: lim, Now: func() time.Time { return now },
		Sessions: fcSessions{{Handle: "cp-aaaa", Account: CopilotAccount, Tool: ToolCopilot, State: "busy", StartedAt: now}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Accounts) != 1 || snap.Accounts[0].Forecast.FiveHourAt != nil {
		t.Errorf("%+v", snap.Accounts)
	}
}

func TestTheReservedAccountIdSurvivesCleaning(t *testing.T) {
	if noEmail(clean(CopilotAccount, maxName)) != CopilotAccount {
		t.Error("mangled")
	}
}

func TestSessionRoutesNeverActOnACopilotSession(t *testing.T) {
	for _, p := range sessionPaths {
		f := &fakeActions{}
		rec := post(sessionEnv(t, f), p, `{"handle":"cp-abcdef012345","account":"personal"}`)
		var out map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		if rec.Code != 409 || out["reason"] != copilotWhy || len(f.calls) != 0 {
			t.Errorf("%s = %d %s calls %v", p, rec.Code, rec.Body, f.calls)
		}
	}
}

func TestIdleRulesForTheCopilotTeamSaveAndLoad(t *testing.T) {
	e := newIdleEnv(t)
	body := `{"default":"break","teamDefault":"desk","accounts":{"~copilot":"hide"},"teams":{},"subtypes":{}}`
	if rec := e.do("POST", body); rec.Code != 200 {
		t.Fatalf("post %d %s", rec.Code, rec.Body)
	}
	if got := e.do("GET", ""); strings.TrimSpace(got.Body.String()) != body {
		t.Errorf("get = %s", got.Body)
	}
	bad := strings.Replace(body, "~copilot", "~other", 1)
	if rec := e.do("POST", bad); rec.Code == 200 {
		t.Error("~other accepted")
	}
}

func TestTheGardenLayoutWithACopilotSessionValidatesAndPinsTheReservedId(t *testing.T) {
	if layout.CopilotAccount != CopilotAccount {
		t.Fatalf("layout %q office %q", layout.CopilotAccount, CopilotAccount)
	}
	g := NewGardenFeed(func() time.Time { return t0 })
	g.Observe(Snapshot{
		Accounts: []AccountOut{{ID: "work", Label: "work"}, {ID: CopilotAccount, Label: "Copilot", Tool: ToolCopilot}},
		Sessions: []SessionOut{{Handle: "a", Account: "work", Tool: ToolClaude}, {Handle: "cp-b", Account: CopilotAccount, Tool: ToolCopilot}},
	})
	l, _, err := g.Layout("rows")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range l.Pavilions {
		found = found || (p.Type == "cabin" && p.Account == CopilotAccount)
	}
	if !found {
		t.Error("no copilot cabin")
	}
	if err := layout.Validate(l, nil); err != nil {
		t.Errorf("Validate: %v", err)
	}
}
