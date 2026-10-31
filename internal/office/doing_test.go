package office

import (
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/office/activity"
)

var dnow = time.Date(2026, 10, 31, 10, 0, 30, 0, time.UTC)

func tool(name, at string, ms int64, ok bool) activity.Entry {
	return activity.Entry{T: at, Kind: activity.KindTool, Tool: name, DurationMs: ms, OK: ok}
}

func result(name, at string) activity.Entry {
	return activity.Entry{T: at, Kind: activity.KindResult, Tool: name, OK: true}
}

func TestToolNameMapsToAKind(t *testing.T) {
	cases := map[string]string{
		"Read": "read", "Grep": "read", "Glob": "read",
		"Edit": "edit", "Write": "edit", "NotebookEdit": "edit", "MultiEdit": "edit",
		"Bash": "run", "WebSearch": "web", "WebFetch": "web",
		"TodoWrite": "other", "mcp__x__y": "other", "": "other",
	}
	for tool, want := range cases {
		if got := doingKind(tool); got != want {
			t.Errorf("doingKind(%q) = %q, want %q", tool, got, want)
		}
	}
}

func TestDoingOfNewestToolCall(t *testing.T) {
	const at = "2026-10-31T10:00:00Z"
	cases := []struct {
		name    string
		entries []activity.Entry // newest first
		want    *Doing
	}{
		{"nothing", nil, nil},
		{"only text", []activity.Entry{{T: at, Kind: activity.KindReply}}, nil},
		{"running", []activity.Entry{tool("Bash", at, 0, true)},
			&Doing{Kind: "run", Tool: "Bash", State: "running", At: time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)}},
		{"done ok", []activity.Entry{result("Edit", "2026-10-31T10:00:02Z"), tool("Edit", at, 2000, true)},
			&Doing{Kind: "edit", Tool: "Edit", State: "ok", At: time.Date(2026, 10, 31, 10, 0, 2, 0, time.UTC)}},
		{"done fail", []activity.Entry{result("Bash", "2026-10-31T10:00:05Z"), tool("Bash", at, 5000, false)},
			&Doing{Kind: "run", Tool: "Bash", State: "fail", At: time.Date(2026, 10, 31, 10, 0, 5, 0, time.UTC)}},
		{"result seen with zero duration is done", []activity.Entry{result("Read", at), tool("Read", at, 0, true)},
			&Doing{Kind: "read", Tool: "Read", State: "ok", At: time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)}},
		{"newest call wins over an older done one", []activity.Entry{tool("WebSearch", at, 0, true), result("Read", at), tool("Read", at, 500, true)},
			&Doing{Kind: "web", Tool: "WebSearch", State: "running", At: time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)}},
		{"subagent start is not the agent's own work", []activity.Entry{{T: at, Kind: activity.KindSubagentStart, Tool: "Agent"}}, nil},
		{"unknown tool is other", []activity.Entry{tool("TodoWrite", at, 0, true)},
			&Doing{Kind: "other", Tool: "TodoWrite", State: "running", At: time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)}},
		{"stale after two minutes", []activity.Entry{tool("Bash", "2026-10-31T09:57:00Z", 0, true)}, nil},
		{"unparseable time is no work", []activity.Entry{tool("Bash", "garbage", 0, true)}, nil},
	}
	for _, c := range cases {
		got := doingOf(c.entries, dnow)
		switch {
		case c.want == nil && got != nil, c.want != nil && (got == nil || *got != *c.want):
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestDoingToolNameIsCleanedAndShort(t *testing.T) {
	long := "mcp__" + string(make([]byte, 0)) + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	d := doingOf([]activity.Entry{tool(long, "2026-10-31T10:00:00Z", 0, true)}, dnow)
	if d == nil || len(d.Tool) > maxName {
		t.Errorf("tool name not capped: %+v", d)
	}
}
