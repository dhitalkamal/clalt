package office

import (
	"time"

	"github.com/dhitalkamal/clalt/internal/office/activity"
)

// doingFresh is how old the newest tool call may be and still count as work.
const doingFresh = 2 * time.Minute

// Doing is what a session or subagent is busy with right now, as the page
// poses it. it holds a kind and a tool name only: no file names, commands or
// text ever leave the parser in it. Kind is read, edit, run, web or other,
// State is running, ok or fail.
type Doing struct {
	Kind  string    `json:"kind"`
	Tool  string    `json:"tool"`
	State string    `json:"state"`
	At    time.Time `json:"at"`
}

// doingKind maps a tool name to a kind by a fixed table.
func doingKind(tool string) string {
	switch tool {
	case "Read", "Grep", "Glob":
		return "read"
	case "Edit", "Write", "NotebookEdit", "MultiEdit":
		return "edit"
	case "Bash":
		return "run"
	case "WebSearch", "WebFetch":
		return "web"
	}
	return "other"
}

// doingOf reads the newest tool call of entries (newest first). nil when there
// is none or it is older than doingFresh. a call is done when it got a
// duration, or a paired result with its tool name came after it.
func doingOf(entries []activity.Entry, now time.Time) *Doing {
	resultTools := map[string]bool{}
	for _, e := range entries {
		switch e.Kind {
		case activity.KindResult:
			if e.Tool != "" {
				resultTools[e.Tool] = true
			}
		case activity.KindTool:
			return doingFromCall(e, resultTools[e.Tool], now)
		}
	}
	return nil
}

func doingFromCall(e activity.Entry, resulted bool, now time.Time) *Doing {
	at, err := time.Parse(time.RFC3339, e.T)
	if err != nil {
		return nil
	}
	d := &Doing{Kind: doingKind(e.Tool), Tool: clean(e.Tool, maxName), State: "running", At: at}
	if e.DurationMs > 0 || resulted {
		d.At = at.Add(time.Duration(e.DurationMs) * time.Millisecond)
		d.State = "ok"
		if !e.OK {
			d.State = "fail"
		}
	}
	if now.Sub(d.At) > doingFresh {
		return nil
	}
	return d
}
