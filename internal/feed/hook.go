package feed

import (
	"encoding/json"
	"time"
)

// HookInput is the part of a hook's stdin that clalt reads. the rest is ignored.
type HookInput struct {
	SessionID        string `json:"session_id"`
	Cwd              string `json:"cwd"`
	Model            string `json:"model"`
	NotificationType string `json:"notification_type"`
	Reason           string `json:"reason"`
	Error            string `json:"error"`
	Source           string `json:"source"`
	AgentID          string `json:"agent_id"`
	AgentType        string `json:"agent_type"`
	ToolName         string `json:"tool_name"`

	// ToolInput is kept raw: only description and subagent_type of an Agent
	// call are ever read from it, see applySubagent.
	ToolInput json.RawMessage `json:"tool_input"`

	// RecentActivity is set by the caller, not read from the input: a tool
	// finished a moment ago, so a permission prompt notification that arrives
	// now (they come about 6 seconds late) is about one that is answered.
	RecentActivity bool `json:"-"`
}

// the notifications that mean claude is blocked on the user. idle_prompt is
// left out on purpose: it would turn a pause into needs-you 60 seconds after
// every reply.
var needsYou = map[string]bool{
	"permission_prompt":      true,
	"elicitation_dialog":     true,
	"elicitation_url_dialog": true,
	"agent_needs_input":      true,
}

// ApplyHook moves the session on for one hook event and says whether anything
// changed, so a caller can skip the write. an ended session stays ended, except
// that a start or a prompt proves it is not: a SessionEnd of reason "other" is
// not always the end of claude.
func ApplyHook(s *SessionState, event string, in HookInput, now time.Time) bool {
	if s.State == StateEnded && event != "SessionStart" && event != "UserPromptSubmit" {
		return false
	}
	switch event {
	case "SessionStart":
		// claude starts a "session" again after an auto compaction, in the middle of a reply
		if in.Source == "compact" {
			return false
		}
		s.setState(StateIdle, now)
		s.Reason, s.LastError = "", ""
		if in.Model != "" {
			s.Model = in.Model
		}
	case "UserPromptSubmit":
		s.setState(StateBusy, now)
		s.Reason, s.LastError = "", ""
	case "Stop":
		// the turn is over, so Agent calls that did not start will not
		s.PendingAgents = nil
		s.setState(StateIdle, now)
		s.Reason, s.LastError = "", ""
	case "StopFailure":
		s.setState(StateIdle, now)
		s.Reason, s.LastError = "", in.Error
		if s.LastError == "" {
			s.LastError = "unknown"
		}
	case "Notification":
		if !needsYou[in.NotificationType] || in.RecentActivity {
			return false
		}
		s.setState(StateNeedsYou, now)
		s.Reason = in.NotificationType
	case "PostToolUse", "PostToolUseFailure":
		if s.State != StateNeedsYou {
			return false
		}
		s.setState(StateBusy, now)
		s.Reason = ""
	case "PreToolUse", "SubagentStart", "SubagentStop":
		// no state change, and no Cwd update: only the subagent list moves
		return applySubagent(s, event, in, now)
	case "SessionEnd":
		s.closeSubagents(now)
		s.PendingAgents = nil
		// /clear and /resume end a conversation, not the session
		if in.Reason == "clear" || in.Reason == "resume" {
			s.setState(StateIdle, now)
		} else {
			s.setState(StateEnded, now)
		}
		s.Reason = ""
	default:
		return false
	}
	if in.Cwd != "" {
		s.Cwd = in.Cwd
	}
	return true
}
