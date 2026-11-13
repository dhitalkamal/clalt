package copilot

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
)

// stateOf maps an event type to the state it puts the session in. a type not
// listed changes nothing. add tool call and permission events here once their
// shapes are captured. permission events are handled in fold, not here.
var stateOf = map[string]feed.State{
	"session.start":        feed.StateIdle,
	"user.message":         feed.StateBusy,
	"model.turn_started":   feed.StateBusy,
	"assistant.turn_start": feed.StateBusy,
	"tool.execution_start": feed.StateBusy,
	"abort":                feed.StateIdle,
	"assistant.turn_end":   feed.StateIdle,
}

// endedType closes a session for good.
const endedType = "session.shutdown"

// reasonPermission is the needs-you reason, same string claude sessions use.
const reasonPermission = "permission_prompt"

// bounds on the open request set so a hostile file cannot grow memory.
const (
	maxOpen  = 64
	maxIDLen = 128
)

// event is one events.jsonl line cut down to what clalt may keep. the free
// text fields are not declared, so json never copies them.
type event struct {
	Type string `json:"type"`
	Data struct {
		StartTime string `json:"startTime"`
		NewModel  string `json:"newModel"`
		Source    string `json:"source"`
		// chosenModel is what auto mode picked
		ChosenModel string `json:"chosenModel"`
		RequestID   string `json:"requestId"`
		Context     struct {
			Cwd string `json:"cwd"`
		} `json:"context"`
	} `json:"data"`
}

// summary is what the tail of an events file says. State, Reason and Model
// are the resolved values, the unexported fields are the fold's working data.
type summary struct {
	State     feed.State
	Reason    string
	Ended     bool
	Model     string
	Cwd       string
	StartedAt time.Time

	base   feed.State          // state ignoring open permission requests
	open   map[string]struct{} // open permission request ids, owned by the cache entry
	named  string              // newest non-agent model_change
	chosen string              // auto mode pick seen after named
}

// view is the summary handed to callers: the open set stays with the cache
// entry, so a later fold cannot change a returned value.
func (s summary) view() summary {
	s.open = nil
	return s
}

// resolve derives State, Reason and Model from the working data.
func (s *summary) resolve() {
	s.State, s.Reason = s.base, ""
	if len(s.open) > 0 {
		s.State, s.Reason = feed.StateNeedsYou, reasonPermission
	}
	s.Model = s.named
	if strings.EqualFold(s.named, "auto") && s.chosen != "" {
		s.Model = s.chosen
	}
}

// fold applies one line to the summary. a line that does not parse is
// skipped.
func (s *summary) fold(line []byte) {
	var e event
	if json.Unmarshal(line, &e) != nil {
		return
	}
	if e.Type == endedType {
		s.Ended = true
		return
	}
	if st, ok := stateOf[e.Type]; ok {
		s.base, s.Ended = st, false
	}
	switch e.Type {
	case "session.start", "abort", "assistant.turn_end":
		clear(s.open)
	}
	switch e.Type {
	case "session.start":
		s.Cwd = e.Data.Context.Cwd
		s.StartedAt, _ = time.Parse(time.RFC3339, e.Data.StartTime)
	case "session.model_change":
		// copilot's internal helpers report as source agent: not the user's model
		if e.Data.Source != "agent" {
			s.named, s.chosen = e.Data.NewModel, ""
		}
	case "session.auto_mode_resolved":
		s.chosen = e.Data.ChosenModel
	case "permission.requested":
		s.base, s.Ended = feed.StateBusy, false
		s.request(e.Data.RequestID)
	case "permission.completed":
		delete(s.open, e.Data.RequestID)
	}
	s.resolve()
}

// request opens a permission request id, dropping it when the set is full.
func (s *summary) request(id string) {
	if id == "" {
		return
	}
	if len(id) > maxIDLen {
		id = id[:maxIDLen]
	}
	if s.open == nil {
		s.open = map[string]struct{}{}
	}
	if _, ok := s.open[id]; ok || len(s.open) < maxOpen {
		s.open[id] = struct{}{}
	}
}
