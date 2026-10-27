package feed

import (
	"encoding/json"
	"strings"
	"time"
	"unicode"
)

const (
	subagentRunning = "running"
	subagentDone    = "done"
	maxSubagents    = 50
	maxPending      = 20
	maxLabelRunes   = 80
)

// Subagent is one subagent of a session, as the office draws it.
type Subagent struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Label     string     `json:"label"`
	State     string     `json:"state"`
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
}

// PendingAgent is a description seen on an Agent tool call that no
// SubagentStart has claimed yet.
type PendingAgent struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	// At is when the call was seen, so a call that never started expires.
	At time.Time `json:"at"`
}

// pendingTTL is how long an Agent call may wait for its SubagentStart. a call
// that was denied or interrupted never starts one.
const pendingTTL = 10 * time.Minute

// cleanLabel drops control and format characters (whitespace ones become a
// space) and caps the result at 80 characters.
func cleanLabel(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t' || r == '\r':
			b.WriteRune(' ')
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r):
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if r := []rune(out); len(r) > maxLabelRunes {
		out = strings.TrimSpace(string(r[:maxLabelRunes]))
	}
	return out
}

// applySubagent handles the subagent events and the Agent tool call. only
// the short description and subagent_type of the tool input are read.
func applySubagent(s *SessionState, event string, in HookInput, now time.Time) bool {
	switch event {
	case "PreToolUse":
		if in.ToolName != "Agent" && in.ToolName != "Task" {
			return false
		}
		var ti struct {
			Description  string `json:"description"`
			SubagentType string `json:"subagent_type"`
		}
		_ = json.Unmarshal(in.ToolInput, &ti)
		s.PendingAgents = append(s.PendingAgents, PendingAgent{Type: cleanLabel(ti.SubagentType), Label: cleanLabel(ti.Description), At: now})
		if len(s.PendingAgents) > maxPending {
			s.PendingAgents = s.PendingAgents[len(s.PendingAgents)-maxPending:]
		}
		return true
	case "SubagentStart":
		id := cleanLabel(in.AgentID)
		if id == "" {
			return false
		}
		a := s.findSubagent(id)
		if a == nil {
			s.Subagents = append(s.Subagents, Subagent{ID: id, StartedAt: now, Label: s.takeLabel(cleanLabel(in.AgentType), now)})
			a = &s.Subagents[len(s.Subagents)-1]
		}
		a.Type, a.State, a.EndedAt = cleanLabel(in.AgentType), subagentRunning, nil
		s.capSubagents()
		return true
	case "SubagentStop":
		id := cleanLabel(in.AgentID)
		if id == "" {
			return false
		}
		a := s.findSubagent(id)
		if a == nil {
			s.Subagents = append(s.Subagents, Subagent{ID: id, Type: cleanLabel(in.AgentType), StartedAt: now})
			a = &s.Subagents[len(s.Subagents)-1]
		}
		a.State, a.EndedAt = subagentDone, &now
		s.capSubagents()
		return true
	}
	return false
}

func (s *SessionState) findSubagent(id string) *Subagent {
	for i := range s.Subagents {
		if s.Subagents[i].ID == id {
			return &s.Subagents[i]
		}
	}
	return nil
}

// takeLabel pops the oldest pending description for a subagent type. a call
// with no subagent_type is the general-purpose agent.
func (s *SessionState) takeLabel(typ string, now time.Time) string {
	live := s.PendingAgents[:0]
	for _, p := range s.PendingAgents {
		if now.Sub(p.At) < pendingTTL {
			live = append(live, p)
		}
	}
	s.PendingAgents = live
	for i, p := range s.PendingAgents {
		if p.Type == typ || (p.Type == "" && typ == "general-purpose") {
			s.PendingAgents = append(s.PendingAgents[:i], s.PendingAgents[i+1:]...)
			return p.Label
		}
	}
	return ""
}

// capSubagents keeps at most 50 records, dropping the oldest done ones first.
func (s *SessionState) capSubagents() {
	for len(s.Subagents) > maxSubagents {
		drop := 0
		for i, a := range s.Subagents {
			if a.State == subagentDone {
				drop = i
				break
			}
		}
		s.Subagents = append(s.Subagents[:drop], s.Subagents[drop+1:]...)
	}
}

// closeSubagents marks every running subagent done, for a session that ended.
func (s *SessionState) closeSubagents(now time.Time) {
	for i := range s.Subagents {
		if s.Subagents[i].State == subagentRunning {
			s.Subagents[i].State, s.Subagents[i].EndedAt = subagentDone, &now
		}
	}
}
