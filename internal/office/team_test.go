package office

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

type fakeTeam struct {
	defs []AgentDef
	got  []string
}

func (f *fakeTeam) Agents(cwds []string) []AgentDef {
	f.got = cwds
	return f.defs
}

type fakeSessions []Session

func (f fakeSessions) Sessions() ([]Session, error) { return f, nil }

type fakeSubs map[string][]Subagent

func (f fakeSubs) Subagents(h string) []Subagent { return f[h] }

func teamBuild(t *testing.T, team TeamSource, sessions []Session, subs map[string][]Subagent) []TeamOut {
	t.Helper()
	snap, err := Build(Sources{
		Accounts: staticAccounts{{ID: "work", Label: "work"}}, Sessions: fakeSessions(sessions),
		Limits: noLimits{}, Subagents: fakeSubs(subs), Team: team, Now: func() time.Time { return t0 },
	})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Team == nil {
		t.Fatal("team is nil, want a list")
	}
	return snap.Team
}

type noLimits struct{}

func (noLimits) Limits(string) (Limits, bool) { return Limits{}, false }

func ids(team []TeamOut) string {
	var out []string
	for _, m := range team {
		out = append(out, m.ID)
	}
	return strings.Join(out, ",")
}

func TestTeamHasTheBuiltinsEvenWithNoSourceAndNoSessions(t *testing.T) {
	for _, src := range []TeamSource{nil, &fakeTeam{}} {
		team := teamBuild(t, src, nil, nil)
		if ids(team) != "general-purpose,explore,plan" {
			t.Errorf("ids %q", ids(team))
		}
		for _, m := range team {
			if m.Source != "builtin" || m.Busy != 0 || m.Desc == "" {
				t.Errorf("%+v", m)
			}
		}
		if team[1].Name != "Explore" || team[2].Name != "Plan" || team[0].Name != "general-purpose" {
			t.Errorf("builtin names %+v", team)
		}
	}
}

func TestTeamEntryJSONShape(t *testing.T) {
	team := teamBuild(t, nil, nil, nil)
	b, _ := json.Marshal(team[1])
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"id", "name", "desc", "source", "busy", "color"} {
		if _, ok := m[k]; !ok {
			t.Errorf("key %s missing in %s", k, b)
		}
	}
	if len(m) != 6 {
		t.Errorf("extra keys in %s", b)
	}
	if !regexp.MustCompile(`^#[0-9a-f]{6}$`).MatchString(team[1].Color) {
		t.Errorf("colour %q", team[1].Color)
	}
	again := teamBuild(t, nil, nil, nil)
	if again[1].Color != team[1].Color {
		t.Error("colour is not stable")
	}
	if team[0].Color == team[1].Color && team[1].Color == team[2].Color {
		t.Error("every member has the same colour")
	}
}

func TestTeamOrderBuiltinThenReviewersThenUserProjectSeen(t *testing.T) {
	src := &fakeTeam{defs: []AgentDef{
		{Name: "zeta", Desc: "z", Source: "project"},
		{Name: "beta", Desc: "b", Source: "user"},
		{Name: "alpha", Desc: "a", Source: "user"},
		{Name: "gamma", Desc: "g", Source: "project"},
	}}
	sess := []Session{{Handle: "h1", Account: "work", Cwd: "/p/one"}}
	subs := map[string][]Subagent{"h1": {
		{ID: "1", Type: "pr-review-toolkit:code-reviewer", State: "done"},
		{ID: "2", Type: "Weird-Plugin:thing", State: "running"},
		{ID: "3", Type: "silent-failure-hunter", State: "running"},
		{ID: "4", Type: "Another", State: "running"},
	}}
	got := ids(teamBuild(t, src, sess, subs))
	want := "general-purpose,explore,plan,pr-review-toolkit:code-reviewer,silent-failure-hunter,alpha,beta,gamma,zeta,another,weird-plugin:thing"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestTeamBusyCountsRunningRunsOfATypeAcrossSessions(t *testing.T) {
	end := t0
	sess := []Session{{Handle: "h1", Account: "work"}, {Handle: "h2", Account: "work"}}
	subs := map[string][]Subagent{
		"h1": {{ID: "1", Type: "Explore", State: "running"}, {ID: "2", Type: "explore", State: "running"}, {ID: "3", Type: "EXPLORE", State: "done"}, {ID: "4", Type: "Explore", State: "running", EndedAt: &end}},
		"h2": {{ID: "5", Type: "explore", State: "running"}, {ID: "6", Type: "general-purpose", State: "running"}, {ID: "7", Type: "Plan", State: "done"}},
	}
	team := teamBuild(t, nil, sess, subs)
	busy := map[string]int{}
	for _, m := range team {
		busy[m.ID] = m.Busy
	}
	if busy["explore"] != 3 || busy["general-purpose"] != 1 || busy["plan"] != 0 {
		t.Errorf("busy %v", busy)
	}
	if len(team) != 3 {
		t.Errorf("a type that differs only by case made a second member: %s", ids(team))
	}
}

func TestTeamCustomAgentKeepsItsDescriptionAndSeenRunsCountOnIt(t *testing.T) {
	src := &fakeTeam{defs: []AgentDef{{Name: "Go-Helper", Desc: "helps with go", Source: "project"}}}
	sess := []Session{{Handle: "h1", Account: "work"}}
	subs := map[string][]Subagent{"h1": {{ID: "1", Type: "go-helper", State: "running"}}}
	team := teamBuild(t, src, sess, subs)
	last := team[len(team)-1]
	if last.ID != "go-helper" || last.Name != "Go-Helper" || last.Source != "project" || last.Busy != 1 || last.Desc != "helps with go" {
		t.Errorf("%+v", last)
	}
}

func TestTeamBuiltinWinsOverACustomAgentOfTheSameName_ProjectWinsOverUser(t *testing.T) {
	src := &fakeTeam{defs: []AgentDef{
		{Name: "explore", Desc: "custom", Source: "user"},
		{Name: "twin", Desc: "from user", Source: "user"},
		{Name: "twin", Desc: "from project", Source: "project"},
	}}
	team := teamBuild(t, src, nil, nil)
	if ids(team) != "general-purpose,explore,plan,twin" {
		t.Fatalf("ids %s", ids(team))
	}
	if team[1].Source != "builtin" || team[3].Source != "project" || team[3].Desc != "from project" {
		t.Errorf("%+v", team)
	}
}

func TestTeamIsCappedAtTwentyFour(t *testing.T) {
	var defs []AgentDef
	for i := 0; i < 40; i++ {
		defs = append(defs, AgentDef{Name: fmt.Sprintf("agent-%02d", i), Source: "user"})
	}
	team := teamBuild(t, &fakeTeam{defs: defs}, nil, nil)
	if len(team) != 24 || team[0].ID != "general-purpose" || team[23].ID != "agent-20" {
		t.Errorf("%d members, last %s", len(team), team[len(team)-1].ID)
	}
}

func TestTeamStringsAreCleanedAndBadIDsSkipped(t *testing.T) {
	src := &fakeTeam{defs: []AgentDef{
		{Name: "mail\x1b[31m@corp.com", Desc: "line1\nline2 \u202e " + strings.Repeat("x", 200) + " @someone", Source: "user"},
		{Name: "bad name", Source: "user"},
		{Name: "under_score", Source: "user"},
		{Name: "", Source: "user"},
		{Name: strings.Repeat("a", 41), Desc: "long name", Source: "user"},
		{Name: "@handle", Desc: "@only", Source: "user"},
	}}
	sess := []Session{{Handle: "h1", Account: "work"}}
	subs := map[string][]Subagent{"h1": {{ID: "1", Type: "has space", State: "running"}, {ID: "2", Type: "x@y.z", State: "running"}}}
	team := teamBuild(t, src, sess, subs)
	for _, m := range team {
		if len(m.Name) > 40 || len(m.Desc) > 120 || strings.ContainsAny(m.Name+m.Desc+m.ID, "\n\x1b@") {
			t.Errorf("not clean: %+v", m)
		}
		if !regexp.MustCompile(`^[a-z0-9:-]{1,64}$`).MatchString(m.ID) {
			t.Errorf("bad id %q", m.ID)
		}
	}
	for _, id := range []string{"bad-name", "under_score", "has-space"} {
		if strings.Contains(","+ids(team)+",", ","+id+",") {
			t.Errorf("%s should be skipped", id)
		}
	}
	if !strings.Contains(ids(team), "mail") {
		t.Errorf("mail agent missing: %s", ids(team))
	}
}

func TestTeamSourceGetsTheRealSessionCwdsBeforeTheyAreCutToAFolderName(t *testing.T) {
	src := &fakeTeam{}
	sess := []Session{{Handle: "h1", Account: "work", Cwd: "/Users/me/proj-a"}, {Handle: "h2", Account: "work", Cwd: "/Users/me/proj-b"}, {Handle: "h3", Account: "work", Cwd: "/Users/me/proj-a"}}
	snap, err := Build(Sources{Accounts: staticAccounts{}, Sessions: fakeSessions(sess), Limits: noLimits{}, Team: src, Now: func() time.Time { return t0 }})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(src.got, ",") != "/Users/me/proj-a,/Users/me/proj-b" {
		t.Errorf("cwds %v", src.got)
	}
	b, _ := json.Marshal(snap)
	if strings.Contains(string(b), "/Users/me") {
		t.Errorf("a full path reached the snapshot: %s", b)
	}
}
