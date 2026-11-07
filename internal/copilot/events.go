package copilot

import (
	"encoding/json"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
)

// stateOf maps an event type to the state it puts the session in. a type not
// listed changes nothing. add tool call and permission events here once their
// shapes are captured (a permission prompt would map to feed.StateNeedsYou).
var stateOf = map[string]feed.State{
	"session.start":        feed.StateIdle,
	"user.message":         feed.StateBusy,
	"model.turn_started":   feed.StateBusy,
	"assistant.turn_start": feed.StateBusy,
	"assistant.turn_end":   feed.StateIdle,
}

// endedType closes a session for good.
const endedType = "session.shutdown"

// event is one events.jsonl line cut down to what clalt may keep. the free
// text fields are not declared, so json never copies them.
type event struct {
	Type string `json:"type"`
	Data struct {
		StartTime string `json:"startTime"`
		NewModel  string `json:"newModel"`
		Context   struct {
			Cwd string `json:"cwd"`
		} `json:"context"`
	} `json:"data"`
}

// summary is what the tail of an events file says.
type summary struct {
	State     feed.State
	Ended     bool
	Model     string
	Cwd       string
	StartedAt time.Time
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
		s.State, s.Ended = st, false
	}
	switch e.Type {
	case "session.start":
		s.Cwd = e.Data.Context.Cwd
		s.StartedAt, _ = time.Parse(time.RFC3339, e.Data.StartTime)
	case "session.model_change":
		s.Model = e.Data.NewModel
	}
}
