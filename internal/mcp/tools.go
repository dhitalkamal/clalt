package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/office"
	"github.com/dhitalkamal/clalt/internal/office/summary"
)

// tool is one read-only tool: its definition and what it runs.
type tool struct {
	name, title, desc string
	props             map[string]any
	run               func(b Backend, args json.RawMessage) (line string, data any, err error)
}

// toolErr is a failure whose text is safe to show the model. anything else
// is logged and shown as a generic message, so paths never leave the machine.
type toolErr string

func (e toolErr) Error() string { return string(e) }

var tools = []tool{
	{name: "limits", title: "Account limits", run: runLimits,
		desc: "Per account: 5h and 7d usage percent, when each window resets, and when the 5h window fills at the current pace. An account with no reading says null, which is not 0 percent."},
	{name: "best_account", title: "Best account now", run: runBest,
		desc: "The account with the most 5h room right now, and why. Accounts with no reading or a used-up week are not picked."},
	{name: "sessions", title: "Running sessions", run: runSessions,
		desc: "Running sessions by account and project with their state (busy, idle, needs-you) and subagent counts. No prompt text."},
	{name: "today", title: "Day summary", run: runToday,
		props: map[string]any{"day": map[string]any{"type": "string", "description": "local day as YYYY-MM-DD, today when left out", "pattern": `^\d{4}-\d{2}-\d{2}$`}},
		desc:  "What each account and project did on a day: sessions, tokens, cost, active minutes, tool calls, failures. No file names or transcript text."},
}

func toolDefs() []any {
	out := make([]any, 0, len(tools))
	for _, t := range tools {
		schema := map[string]any{"type": "object", "additionalProperties": false}
		schema["properties"] = map[string]any{}
		if t.props != nil {
			schema["properties"] = t.props
		}
		out = append(out, map[string]any{
			"name": t.name, "title": t.title, "description": t.desc, "inputSchema": schema,
			"annotations": map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		})
	}
	return out
}

// callTool runs a tools/call. a bad request is a protocol error, a tool that
// fails is a result with isError so the model can read it.
func callTool(params json.RawMessage, b Backend) (map[string]any, *rpcError) {
	var p struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.Name == "" {
		return nil, &rpcError{Code: codeBadParams, Message: "tools/call needs a tool name"}
	}
	for _, t := range tools {
		if t.name != p.Name {
			continue
		}
		line, data, err := t.run(b, p.Args)
		if err != nil {
			msg := "could not read the clalt data"
			var te toolErr
			if errors.As(err, &te) {
				msg = te.Error()
			} else {
				b.logf("%s failed: %v", t.name, err)
			}
			return map[string]any{"content": []any{text(msg)}, "isError": true}, nil
		}
		body, err := json.Marshal(data)
		if err != nil {
			b.logf("%s encode failed: %v", t.name, err)
			return map[string]any{"content": []any{text("could not encode the reply")}, "isError": true}, nil
		}
		return map[string]any{"content": []any{text(line), text(string(body))}, "isError": false}, nil
	}
	return nil, &rpcError{Code: codeBadParams, Message: "unknown tool: " + one(p.Name)}
}

func text(s string) map[string]any { return map[string]any{"type": "text", "text": s} }

// one keeps a client-chosen name to a short single line before echoing it.
func one(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}

// ---- limits ----

type windowView struct {
	UsedPct  float64   `json:"usedPct"`
	LeftPct  float64   `json:"leftPct"`
	ResetsAt time.Time `json:"resetsAt"`
	ResetsIn string    `json:"resetsIn"`
}

type limitsView struct {
	ID       string             `json:"id"`
	Label    string             `json:"label"`
	FiveHour *windowView        `json:"fiveHour"`
	SevenDay *windowView        `json:"sevenDay"`
	ReadAt   *time.Time         `json:"readAt"`
	Forecast office.ForecastOut `json:"forecast"`
}

func view(w *office.WindowOut, now time.Time) *windowView {
	if w == nil {
		return nil
	}
	return &windowView{UsedPct: w.UsedPct, LeftPct: round1(100 - w.UsedPct), ResetsAt: w.ResetsAt, ResetsIn: dur(w.ResetsAt.Sub(now))}
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }

func accountViews(b Backend) ([]limitsView, error) {
	snap, err := b.Snapshot()
	if err != nil {
		return nil, err
	}
	now := b.now()
	out := make([]limitsView, 0, len(snap.Accounts))
	for _, a := range snap.Accounts {
		out = append(out, limitsView{
			ID: office.NoEmail(a.ID), Label: office.NoEmail(a.Label),
			FiveHour: view(a.Limits.FiveHour, now), SevenDay: view(a.Limits.SevenDay, now),
			ReadAt: a.Limits.ReadAt, Forecast: a.Forecast,
		})
	}
	return out, nil
}

func runLimits(b Backend, _ json.RawMessage) (string, any, error) {
	views, err := accountViews(b)
	if err != nil {
		return "", nil, err
	}
	if len(views) == 0 {
		return "no accounts", map[string]any{"now": b.now().UTC(), "accounts": views}, nil
	}
	parts := make([]string, 0, len(views))
	for _, v := range views {
		if v.FiveHour == nil && v.SevenDay == nil {
			parts = append(parts, v.Label+" no reading")
			continue
		}
		s := v.Label
		if v.FiveHour != nil {
			s += fmt.Sprintf(" 5h %s%% (resets in %s", pct(v.FiveHour.UsedPct), v.FiveHour.ResetsIn)
			if v.Forecast.FiveHourAt != nil {
				s += ", full in " + dur(v.Forecast.FiveHourAt.Sub(b.now()))
			}
			s += ")"
		}
		if v.SevenDay != nil {
			s += fmt.Sprintf(" 7d %s%%", pct(v.SevenDay.UsedPct))
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, "; "), map[string]any{"now": b.now().UTC(), "accounts": views}, nil
}

// ---- best account ----

type bestView struct {
	ID               string    `json:"id"`
	Label            string    `json:"label"`
	FiveHourLeftPct  float64   `json:"fiveHourLeftPct"`
	FiveHourResetsAt time.Time `json:"fiveHourResetsAt"`
	SevenDayUsedPct  *float64  `json:"sevenDayUsedPct"`
}

func runBest(b Backend, _ json.RawMessage) (string, any, error) {
	views, err := accountViews(b)
	if err != nil {
		return "", nil, err
	}
	var best *limitsView
	for i := range views {
		v := &views[i]
		// unknown is not room, and a used-up week blocks the account whatever the 5h says
		if v.FiveHour == nil || (v.SevenDay != nil && v.SevenDay.UsedPct >= 100) {
			continue
		}
		if best == nil || better(v, best) {
			best = v
		}
	}
	if best == nil {
		why := "no account has a 5h reading with room, so there is no pick"
		return why, map[string]any{"best": nil, "why": why}, nil
	}
	bv := bestView{ID: best.ID, Label: best.Label, FiveHourLeftPct: best.FiveHour.LeftPct, FiveHourResetsAt: best.FiveHour.ResetsAt}
	why := fmt.Sprintf("%s has the most 5h room: %s%% left, resets in %s", best.Label, pct(best.FiveHour.LeftPct), best.FiveHour.ResetsIn)
	if best.SevenDay != nil {
		u := best.SevenDay.UsedPct
		bv.SevenDayUsedPct = &u
		why += fmt.Sprintf("; 7d is %s%% used", pct(u))
	}
	return why, map[string]any{"best": bv, "why": why}, nil
}

// better is more 5h room, then a lighter week, then the label.
func better(a, b *limitsView) bool {
	if a.FiveHour.UsedPct != b.FiveHour.UsedPct {
		return a.FiveHour.UsedPct < b.FiveHour.UsedPct
	}
	aw, bw := weekUsed(a), weekUsed(b)
	if aw != bw {
		return aw < bw
	}
	return a.Label < b.Label
}

func weekUsed(v *limitsView) float64 {
	if v.SevenDay == nil {
		return 100 // an unknown week loses a tie
	}
	return v.SevenDay.UsedPct
}

// ---- sessions ----

type subView struct {
	Total   int `json:"total"`
	Running int `json:"running"`
}

type sessionView struct {
	Handle         string    `json:"handle"`
	Account        string    `json:"account"`
	Project        string    `json:"project"`
	Model          string    `json:"model,omitempty"`
	State          string    `json:"state"`
	NeedsYouReason string    `json:"needsYouReason,omitempty"`
	StartedAt      time.Time `json:"startedAt"`
	Subagents      subView   `json:"subagents"`
}

func runSessions(b Backend, _ json.RawMessage) (string, any, error) {
	snap, err := b.Snapshot()
	if err != nil {
		return "", nil, err
	}
	list := make([]sessionView, 0, len(snap.Sessions))
	states := map[string]int{}
	running := 0
	for _, s := range snap.Sessions {
		v := sessionView{Handle: s.Handle, Account: office.NoEmail(s.Account), Project: s.Project, Model: s.Model,
			State: s.State, NeedsYouReason: s.NeedsYouReason, StartedAt: s.StartedAt}
		for _, sub := range s.Subagents {
			v.Subagents.Total++
			if sub.State == "running" {
				v.Subagents.Running++
			}
		}
		running += v.Subagents.Running
		states[s.State]++
		list = append(list, v)
	}
	data := map[string]any{"sessions": list}
	if len(list) == 0 {
		return "no running sessions", data, nil
	}
	var parts []string
	for _, st := range []string{"busy", "idle", "needs-you", "starting", "ended"} {
		if states[st] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", states[st], st))
		}
	}
	line := fmt.Sprintf("%d %s: %s", len(list), plural(len(list), "session"), strings.Join(parts, ", "))
	if running > 0 {
		line += fmt.Sprintf("; %d %s running", running, plural(running, "subagent"))
	}
	return line, data, nil
}

func plural(n int, w string) string {
	if n == 1 {
		return w
	}
	return w + "s"
}

// ---- today ----

type statsView struct {
	Sessions      int             `json:"sessions"`
	Tokens        int64           `json:"tokens"`
	Cost          float64         `json:"cost"`
	ActiveMinutes int             `json:"activeMinutes"`
	ToolCalls     int             `json:"toolCalls"`
	FailureCount  int             `json:"failureCount"`
	TopTools      []summary.Count `json:"topTools"`
}

type projectView struct {
	Name string `json:"name"`
	statsView
}

type dayAccountView struct {
	ID       string        `json:"id"`
	Label    string        `json:"label"`
	Projects []projectView `json:"projects"`
	statsView
}

func statsOf(s summary.Stats) statsView {
	top := s.Tools
	if len(top) > 5 {
		top = top[:5]
	}
	if top == nil {
		top = []summary.Count{}
	}
	return statsView{Sessions: s.Sessions, Tokens: s.Tokens, Cost: float64(int(s.Cost*100+0.5)) / 100, ActiveMinutes: s.ActiveMinutes,
		ToolCalls: s.ToolCalls, FailureCount: s.FailureCount, TopTools: top}
}

func runToday(b Backend, args json.RawMessage) (string, any, error) {
	var a struct {
		Day string `json:"day"`
	}
	if len(args) > 0 && string(args) != "null" {
		if err := json.Unmarshal(args, &a); err != nil {
			return "", nil, toolErr("day must be a string like 2026-10-28")
		}
	}
	if a.Day != "" {
		if t, err := time.Parse("2006-01-02", a.Day); err != nil || t.Format("2006-01-02") != a.Day {
			return "", nil, toolErr("day must be YYYY-MM-DD")
		}
	}
	sum, err := b.Day(a.Day)
	if errors.Is(err, summary.ErrBadDay) {
		return "", nil, toolErr("day must be YYYY-MM-DD")
	}
	if err != nil {
		return "", nil, err
	}
	accts := make([]dayAccountView, 0, len(sum.Accounts))
	for _, acc := range sum.Accounts {
		v := dayAccountView{ID: office.NoEmail(acc.ID), Label: office.NoEmail(acc.Label), Projects: []projectView{}, statsView: statsOf(acc.Stats)}
		for _, p := range acc.Projects {
			v.Projects = append(v.Projects, projectView{Name: p.Name, statsView: statsOf(p.Stats)})
		}
		accts = append(accts, v)
	}
	tot := statsOf(sum.Totals)
	data := map[string]any{"day": sum.Day, "truncated": sum.Truncated, "totals": tot, "accounts": accts}
	line := fmt.Sprintf("%s: %s tokens, $%.2f, %d %s, %d active minutes across %d %s", sum.Day, tokens(tot.Tokens), tot.Cost,
		tot.Sessions, plural(tot.Sessions, "session"), tot.ActiveMinutes, len(accts), plural(len(accts), "account"))
	if sum.Truncated {
		line += " (partial: the transcripts were too large to read in full)"
	}
	return line, data, nil
}

// ---- small formatters ----

func pct(f float64) string {
	if f == float64(int(f)) {
		return fmt.Sprintf("%d", int(f))
	}
	return fmt.Sprintf("%.1f", f)
}

// dur is a short span like 2h10m, 45m or 3d4h; negative is now.
func dur(d time.Duration) string {
	if d < time.Minute {
		return "under a minute"
	}
	m := int(d.Minutes())
	switch {
	case m >= 48*60:
		return fmt.Sprintf("%dd%dh", m/(24*60), m%(24*60)/60)
	case m >= 60:
		return fmt.Sprintf("%dh%02dm", m/60, m%60)
	}
	return fmt.Sprintf("%dm", m)
}

func tokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprintf("%d", n)
}
