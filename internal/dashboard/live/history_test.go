package live

import (
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/usage"
)

var noon = time.Date(2026, time.October, 23, 12, 0, 0, 0, time.UTC) // a Friday

func rec(daysAgo int, project, cwd, model, session string, in, out int64) usage.Record {
	return usage.Record{
		Timestamp: noon.AddDate(0, 0, -daysAgo).Add(time.Duration(len(model)) * time.Minute),
		Model:     model, Project: project, Cwd: cwd, SessionID: session,
		Input: in, Output: out, CacheRead: 10, CacheCreate: 5,
	}
}

func TestTheDaysAreThirtyNewestFirstWithTheirSums(t *testing.T) {
	l := buildLedger(map[string][]usage.Record{"a": {
		rec(0, "p", "/h/p", "claude-opus-5-5", "s1", 100, 50),
		rec(0, "p", "/h/p", "claude-opus-5-5", "s1", 100, 50),
		rec(0, "p", "/h/p", "claude-opus-5-5", "s2", 1, 1),
		rec(2, "p", "/h/p", "claude-opus-5-5", "s3", 7, 3),
		rec(30, "p", "/h/p", "claude-opus-5-5", "s4", 9, 9), // outside the window
	}}, noon, "/h")

	days := l.histories["a"].Days
	if len(days) != 30 || days[0].Label != "Fri 23" || days[2].Label != "Wed 21" {
		t.Fatalf("days = %d, first %q, third %q", len(days), days[0].Label, days[2].Label)
	}
	d := days[0]
	if d.Input != 201 || d.Output != 101 || d.CacheRead != 30 || d.CacheWrite != 15 || d.Sessions != 2 || d.Cost <= 0 {
		t.Errorf("today = %+v", d)
	}
	if days[2].Sessions != 1 || days[1].Sessions != 0 {
		t.Errorf("sessions of the other days: %+v %+v", days[1], days[2])
	}
}

func TestAnAccountWithNoRecordsHasNoHistory(t *testing.T) {
	l := buildLedger(map[string][]usage.Record{"a": nil}, noon, "/h")
	if h := l.histories["a"]; h.Days != nil || h.Projects != nil {
		t.Errorf("history = %+v", h)
	}
}

func TestProjectsAreNamedByTheirFolderAndStayApart(t *testing.T) {
	l := buildLedger(map[string][]usage.Record{"a": {
		rec(0, "-h-work-api", "/h/work/api", "claude-opus-5-5", "s1", 100, 0),
		rec(0, "-h-me-api", "/h/me/api", "claude-opus-5-5", "s2", 50, 0),
		rec(0, "-h-clalt", "/h/clalt", "claude-opus-5-5", "s3", 10, 0),
		rec(0, "-x", "", "claude-opus-5-5", "s4", 1, 0),
	}}, noon, "/h")

	var names []string
	for _, p := range l.histories["a"].Projects {
		names = append(names, p.Name)
	}
	want := []string{"work/api", "me/api", "clalt", "-x"}
	if len(names) != 4 || names[0] != want[0] || names[1] != want[1] || names[2] != want[2] || names[3] != want[3] {
		t.Errorf("names = %v, want %v (biggest cost first)", names, want)
	}
}

func TestAProjectListsItsActivityNewestDayFirstThenCostliest(t *testing.T) {
	l := buildLedger(map[string][]usage.Record{"a": {
		rec(1, "p", "/h/p", "claude-sonnet-5", "s1", 10, 0),
		rec(0, "p", "/h/p", "claude-sonnet-5", "s2", 10, 0),
		rec(0, "p", "/h/p", "claude-opus-5-5", "s3", 1000, 0),
	}}, noon, "/h")

	p := l.histories["a"].Projects[0]
	if p.Name != "p" || p.Sessions != 3 || len(p.Activity) != 3 {
		t.Fatalf("project = %+v", p)
	}
	got := []string{p.Activity[0].Day + " " + p.Activity[0].Model, p.Activity[1].Model, p.Activity[2].Day}
	if got[0] != "Fri 23 Opus 5.5" || got[1] != "Sonnet 5" || got[2] != "Thu 22" {
		t.Errorf("activity = %v", got)
	}
}

func TestModelsAreMergedByTheirShownName(t *testing.T) {
	l := buildLedger(map[string][]usage.Record{"a": {
		rec(0, "p", "/h/p", "claude-opus-5-5-20260101", "s1", 10, 5),
		rec(1, "p", "/h/p", "claude-opus-5-5", "s2", 10, 5),
		rec(1, "p", "/h/p", "claude-sonnet-5", "s2", 1, 1),
	}}, noon, "/h")

	m := l.histories["a"].Models
	if len(m) != 2 || m[0].Name != "Opus 5.5" || m[0].Tokens != 30 || m[1].Name != "Sonnet 5" {
		t.Errorf("models = %+v", m)
	}
}

func TestUnattributedUsageIsCountedAndSessionsAreCostedAcrossAccounts(t *testing.T) {
	l := buildLedger(map[string][]usage.Record{
		"a":                {rec(0, "p", "/h/p", "claude-opus-5-5", "s1", 1000, 100)},
		usage.Unattributed: {rec(1, "p", "/h/p", "claude-opus-5-5", "s2", 1000, 100), rec(40, "p", "/h/p", "claude-opus-5-5", "s9", 1000, 100)},
	}, noon, "/h")

	if _, ok := l.histories[usage.Unattributed]; ok {
		t.Error("unattributed usage is not an account")
	}
	if l.unattributed <= 0 || l.sessionCost["s2"] != l.unattributed || l.sessionCost["s1"] <= 0 {
		t.Errorf("unattributed %v, session costs %v", l.unattributed, l.sessionCost)
	}
	if _, old := l.sessionCost["s9"]; old {
		t.Error("a session older than the window is not costed")
	}
}

func TestTheLastFolderIsThatOfTheNewestRecordWithHomeAsTilde(t *testing.T) {
	l := buildLedger(map[string][]usage.Record{"a": {
		rec(3, "p", "/h/old", "claude-opus-5-5", "s1", 1, 1),
		rec(0, "q", "/h/proj/new", "claude-opus-5-5", "s2", 1, 1),
	}}, noon, "/h")
	if got := l.lastIn["a"]; got != "~/proj/new" {
		t.Errorf("last in = %q", got)
	}
}

func TestADayIsADayInTheClocksZone(t *testing.T) {
	zone := time.FixedZone("x", 10*3600)
	now := time.Date(2026, 10, 23, 1, 0, 0, 0, zone) // 15:00 on the 22nd in UTC
	r := usage.Record{Timestamp: now.Add(-2 * time.Hour), Model: "claude-opus-5-5", Project: "p", SessionID: "s", Input: 5}
	l := buildLedger(map[string][]usage.Record{"a": {r}}, now, "/h")
	if d := l.histories["a"].Days; d[0].Input != 0 || d[1].Input != 5 {
		t.Errorf("the record is from yesterday there: %+v %+v", d[0], d[1])
	}
}

func TestModelNames(t *testing.T) {
	for id, want := range map[string]string{
		"claude-opus-5-5":           "Opus 5.5",
		"claude-opus-5-5-20260101":  "Opus 5.5",
		"claude-sonnet-5":           "Sonnet 5",
		"claude-3-5-haiku-20241022": "Haiku 3.5",
		"claude-opus-4-1[1m]":       "Opus 4.1",
		"gpt-x":                     "gpt-x",
		"":                          "unknown",
		"<synthetic>":               "<synthetic>",
	} {
		if got := modelName(id); got != want {
			t.Errorf("modelName(%q) = %q, want %q", id, got, want)
		}
	}
}
