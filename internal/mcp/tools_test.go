package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/office"
	"github.com/dhitalkamal/clalt/internal/office/summary"
)

var t0 = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

func win(pct float64, in time.Duration) *office.WindowOut {
	return &office.WindowOut{UsedPct: pct, ResetsAt: t0.Add(in)}
}

// fake is a backend over made-up accounts: work is busy, play has room.
func fake() Backend {
	fill := t0.Add(90 * time.Minute)
	snap := office.Snapshot{
		V: 1,
		Accounts: []office.AccountOut{
			{ID: "acc-1", Label: "play", Limits: office.LimitsOut{FiveHour: win(20, 4*time.Hour), SevenDay: win(10, 100*time.Hour)}},
			{ID: "acc-2", Label: "work", Limits: office.LimitsOut{FiveHour: win(70, time.Hour), SevenDay: win(40, 50*time.Hour)},
				Forecast: office.ForecastOut{FiveHourAt: &fill, PacePctPerHour: 22.5}},
			{ID: "acc-3", Label: "idle-acct"},
		},
		Sessions: []office.SessionOut{
			{Handle: "h1", Account: "work", Project: "api", Model: "opus", State: "busy", StartedAt: t0,
				Subagents: []office.Subagent{{ID: "s1", Type: "explore", Label: "secret prompt text", State: "running"}, {ID: "s2", State: "done"}}},
			{Handle: "h2", Account: "play", Project: "site", State: "needs-you", NeedsYouReason: "permission_prompt", StartedAt: t0, Subagents: []office.Subagent{}},
			{Handle: "h3", Account: "play", Project: "site", State: "idle", StartedAt: t0, Subagents: []office.Subagent{}},
		},
	}
	return Backend{
		Now:      func() time.Time { return t0 },
		Snapshot: func() (office.Snapshot, error) { return snap, nil },
		Day: func(day string) (summary.Summary, error) {
			if day == "" {
				day = "2026-10-28"
			}
			if day == "boom" {
				return summary.Summary{}, errors.New("disk on fire at /Users/me/secret")
			}
			st := summary.Stats{Sessions: 2, Tokens: 1200, Cost: 1.5, ActiveMinutes: 30, ToolCalls: 9, FailureCount: 1,
				Files:    []summary.Count{{Name: "private.txt", Count: 3}},
				Failures: []summary.Failure{{Tool: "Bash", Text: "secret failure text", Count: 1}},
				Tools:    []summary.Count{{Name: "Read", Count: 5}}}
			return summary.Summary{V: 1, Day: day, Totals: st, Accounts: []summary.AccountOut{
				{ID: "acc-2", Label: "me@example.com", Stats: st, Projects: []summary.ProjectOut{{Name: "api", Stats: st}}},
			}}, nil
		},
	}
}

// call runs one tools/call and returns the result map.
func call(t *testing.T, b Backend, name, args string) map[string]any {
	t.Helper()
	line := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, name, args)
	got := talk(t, b, line)
	if len(got) != 1 || got[0].Error != nil {
		t.Fatalf("call %s: %+v", name, got)
	}
	return got[0].Result
}

// texts is the text of each content item of a tool result.
func texts(t *testing.T, res map[string]any) []string {
	t.Helper()
	var out []string
	for _, c := range res["content"].([]any) {
		m := c.(map[string]any)
		if m["type"] != "text" {
			t.Fatalf("content type = %v", m["type"])
		}
		out = append(out, m["text"].(string))
	}
	return out
}

// payload is the JSON item of a tool result, the summary line being the first.
func payload(t *testing.T, res map[string]any) map[string]any {
	t.Helper()
	tx := texts(t, res)
	if len(tx) != 2 || strings.Contains(tx[0], "\n") {
		t.Fatalf("content = %q, want a one-line summary then json", tx)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(tx[1]), &m); err != nil {
		t.Fatalf("second item is not json: %v", err)
	}
	if strings.Contains(tx[1], "\n") {
		t.Errorf("json is not compact")
	}
	return m
}

func TestToolsListIsTheReadOnlySet(t *testing.T) {
	got := talk(t, fake(), `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	tools := got[0].Result["tools"].([]any)
	var names []string
	for _, x := range tools {
		m := x.(map[string]any)
		names = append(names, m["name"].(string))
		ann := m["annotations"].(map[string]any)
		if ann["readOnlyHint"] != true || ann["destructiveHint"] != false {
			t.Errorf("%v annotations = %v, want read only", m["name"], ann)
		}
		sch := m["inputSchema"].(map[string]any)
		if sch["type"] != "object" {
			t.Errorf("%v inputSchema = %v", m["name"], sch)
		}
		if m["description"] == "" {
			t.Errorf("%v has no description", m["name"])
		}
	}
	sort.Strings(names)
	want := []string{"best_account", "limits", "sessions", "today"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("tools = %v, want exactly %v (read-only, nothing that writes)", names, want)
	}
}

func TestTodayInputSchemaTakesAnOptionalDay(t *testing.T) {
	got := talk(t, fake(), `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	for _, x := range got[0].Result["tools"].([]any) {
		m := x.(map[string]any)
		sch := m["inputSchema"].(map[string]any)
		props, _ := sch["properties"].(map[string]any)
		if m["name"] == "today" {
			day := props["day"].(map[string]any)
			if day["type"] != "string" || sch["required"] != nil {
				t.Errorf("today schema = %v", sch)
			}
		} else if len(props) != 0 || sch["additionalProperties"] != false {
			t.Errorf("%v schema = %v, want no arguments", m["name"], sch)
		}
	}
}

func TestLimitsReportsEachAccountAndTheForecast(t *testing.T) {
	res := call(t, fake(), "limits", `{}`)
	if res["isError"] == true {
		t.Fatalf("error: %v", res)
	}
	p := payload(t, res)
	accts := p["accounts"].([]any)
	if len(accts) != 3 {
		t.Fatalf("accounts = %d", len(accts))
	}
	work := accts[1].(map[string]any)
	if work["label"] != "work" || work["fiveHour"].(map[string]any)["usedPct"] != 70.0 {
		t.Errorf("work = %v", work)
	}
	if work["forecast"].(map[string]any)["fiveHourAt"] == nil {
		t.Errorf("forecast lost: %v", work)
	}
	if accts[2].(map[string]any)["fiveHour"] != nil {
		t.Errorf("an account with no reading must say null, not 0: %v", accts[2])
	}
	line := texts(t, res)[0]
	for _, want := range []string{"play", "work", "70%", "no reading"} {
		if !strings.Contains(line, want) {
			t.Errorf("summary %q lacks %q", line, want)
		}
	}
}

func TestBestAccountPicksTheMostFiveHourRoom(t *testing.T) {
	res := call(t, fake(), "best_account", `{}`)
	p := payload(t, res)
	best := p["best"].(map[string]any)
	if best["label"] != "play" || best["fiveHourLeftPct"] != 80.0 {
		t.Errorf("best = %v", best)
	}
	if !strings.Contains(p["why"].(string), "80%") {
		t.Errorf("why = %v", p["why"])
	}
}

func TestBestAccountSkipsAWeekExhaustedAccount(t *testing.T) {
	b := fake()
	snap, _ := b.Snapshot()
	snap.Accounts[0].Limits.SevenDay = win(100, 10*time.Hour)
	b.Snapshot = func() (office.Snapshot, error) { return snap, nil }
	best := payload(t, call(t, b, "best_account", `{}`))["best"].(map[string]any)
	if best["label"] != "work" {
		t.Errorf("best = %v, want work since play has used its week", best)
	}
}

func TestBestAccountWithNoReadingsSaysSo(t *testing.T) {
	b := Backend{Now: fake().Now, Snapshot: func() (office.Snapshot, error) {
		return office.Snapshot{Accounts: []office.AccountOut{{ID: "a", Label: "solo"}}}, nil
	}}
	res := call(t, b, "best_account", `{}`)
	p := payload(t, res)
	if p["best"] != nil || res["isError"] == true {
		t.Errorf("reply = %v", p)
	}
	if !strings.Contains(texts(t, res)[0], "no") {
		t.Errorf("summary = %q", texts(t, res)[0])
	}
}

func TestSessionsCountsStatesAndSubagentsWithoutText(t *testing.T) {
	res := call(t, fake(), "sessions", `{}`)
	raw := strings.Join(texts(t, res), "\n")
	if strings.Contains(raw, "secret prompt text") {
		t.Errorf("a subagent label leaked: %s", raw)
	}
	p := payload(t, res)
	list := p["sessions"].([]any)
	if len(list) != 3 {
		t.Fatalf("sessions = %d", len(list))
	}
	first := list[0].(map[string]any)
	sub := first["subagents"].(map[string]any)
	if first["project"] != "api" || first["state"] != "busy" || sub["running"] != 1.0 || sub["total"] != 2.0 {
		t.Errorf("first = %v", first)
	}
	if list[1].(map[string]any)["needsYouReason"] != "permission_prompt" {
		t.Errorf("needs-you reason missing: %v", list[1])
	}
	line := texts(t, res)[0]
	if !strings.Contains(line, "3 sessions") || !strings.Contains(line, "1 needs-you") || !strings.Contains(line, "1 subagent running") {
		t.Errorf("summary = %q", line)
	}
}

func TestSessionsWithNoneRunning(t *testing.T) {
	b := Backend{Snapshot: func() (office.Snapshot, error) { return office.Snapshot{}, nil }}
	res := call(t, b, "sessions", `{}`)
	if !strings.Contains(texts(t, res)[0], "no running sessions") {
		t.Errorf("summary = %q", texts(t, res)[0])
	}
	if payload(t, res)["sessions"] == nil {
		t.Errorf("sessions must be [] not null")
	}
}

func TestTodayIsCompactAndDropsFilesAndFailureText(t *testing.T) {
	res := call(t, fake(), "today", `{}`)
	raw := strings.Join(texts(t, res), "\n")
	for _, bad := range []string{"private.txt", "secret failure text", "me@example.com"} {
		if strings.Contains(raw, bad) {
			t.Errorf("today leaked %q: %s", bad, raw)
		}
	}
	p := payload(t, res)
	acc := p["accounts"].([]any)[0].(map[string]any)
	if acc["label"] != "me" || acc["tokens"] != 1200.0 || acc["projects"].([]any)[0].(map[string]any)["name"] != "api" {
		t.Errorf("account = %v", acc)
	}
	if p["day"] != "2026-10-28" {
		t.Errorf("day = %v", p["day"])
	}
}

func TestTodayPassesTheDayThrough(t *testing.T) {
	p := payload(t, call(t, fake(), "today", `{"day":"2026-10-01"}`))
	if p["day"] != "2026-10-01" {
		t.Errorf("day = %v", p["day"])
	}
}

func TestToolFailuresAreToolErrorsNotProtocolErrors(t *testing.T) {
	cases := []struct{ name, tool, args string }{
		{"bad day", "today", `{"day":"yesterday"}`},
		{"day of the wrong type", "today", `{"day":5}`},
		{"backend error", "today", `{"day":"boom"}`},
	}
	for _, c := range cases {
		res := call(t, fake(), c.tool, c.args)
		if res["isError"] != true {
			t.Errorf("%s: isError = %v, want true: %v", c.name, res["isError"], res)
			continue
		}
		if tx := strings.Join(texts(t, res), ""); strings.Contains(tx, "/Users/me") {
			t.Errorf("%s: the error leaked a path: %s", c.name, tx)
		}
	}
	b := Backend{Snapshot: func() (office.Snapshot, error) { return office.Snapshot{}, errors.New("no state at /home/x") }}
	for _, tool := range []string{"limits", "best_account", "sessions"} {
		res := call(t, b, tool, `{}`)
		if res["isError"] != true || strings.Contains(strings.Join(texts(t, res), ""), "/home/x") {
			t.Errorf("%s: %v", tool, res)
		}
	}
}

func TestUnknownToolAndBadParamsAreProtocolErrors(t *testing.T) {
	got := talk(t, fake(),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"delete_account","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":"nope"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{}}`)
	for i, r := range got {
		if r.Error == nil || r.Error.Code != -32602 {
			t.Errorf("reply %d = %+v, want -32602", i, r)
		}
	}
}

func TestNoToolEverPrintsAnEmail(t *testing.T) {
	b := fake()
	snap, _ := b.Snapshot()
	snap.Accounts[0].Label = "me@example.com"
	b.Snapshot = func() (office.Snapshot, error) { return snap, nil }
	for _, tool := range []string{"limits", "best_account"} {
		tx := strings.Join(texts(t, call(t, b, tool, `{}`)), "")
		if strings.Contains(tx, "@") {
			t.Errorf("%s printed an at sign: %s", tool, tx)
		}
	}
}
