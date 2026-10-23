package feed

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

func start() SessionState {
	return SessionState{Handle: "h", Account: "work", State: StateStarting, Since: t0.Add(-time.Minute)}
}

func TestApplyHookTransitions(t *testing.T) {
	cases := []struct {
		name   string
		from   SessionState
		event  string
		in     HookInput
		want   State
		reason string
		lastEr string
		write  bool
	}{
		{"start is idle", start(), "SessionStart", HookInput{Model: "claude-opus-5", Cwd: "/c"}, StateIdle, "", "", true},
		{"prompt is busy", SessionState{State: StateIdle, LastError: "rate_limit"}, "UserPromptSubmit", HookInput{}, StateBusy, "", "", true},
		{"stop is idle", SessionState{State: StateBusy}, "Stop", HookInput{}, StateIdle, "", "", true},
		{"stop failure keeps the error", SessionState{State: StateBusy}, "StopFailure", HookInput{Error: "rate_limit"}, StateIdle, "", "rate_limit", true},
		{"permission prompt needs you", SessionState{State: StateBusy}, "Notification", HookInput{NotificationType: "permission_prompt"}, StateNeedsYou, "permission_prompt", "", true},
		{"elicitation needs you", SessionState{State: StateBusy}, "Notification", HookInput{NotificationType: "elicitation_dialog"}, StateNeedsYou, "elicitation_dialog", "", true},
		{"idle prompt changes nothing", SessionState{State: StateIdle}, "Notification", HookInput{NotificationType: "idle_prompt"}, StateIdle, "", "", false},
		{"tool result clears needs-you", SessionState{State: StateNeedsYou, Reason: "permission_prompt"}, "PostToolUse", HookInput{}, StateBusy, "", "", true},
		{"tool failure clears needs-you", SessionState{State: StateNeedsYou, Reason: "permission_prompt"}, "PostToolUseFailure", HookInput{}, StateBusy, "", "", true},
		{"tool result while busy writes nothing", SessionState{State: StateBusy}, "PostToolUse", HookInput{}, StateBusy, "", "", false},
		{"tool result while idle writes nothing", SessionState{State: StateIdle}, "PostToolUse", HookInput{}, StateIdle, "", "", false},
		{"clear keeps the session", SessionState{State: StateBusy}, "SessionEnd", HookInput{Reason: "clear"}, StateIdle, "", "", true},
		{"resume keeps the session", SessionState{State: StateBusy}, "SessionEnd", HookInput{Reason: "resume"}, StateIdle, "", "", true},
		{"exit ends it", SessionState{State: StateIdle}, "SessionEnd", HookInput{Reason: "prompt_input_exit"}, StateEnded, "", "", true},
		{"an unknown reason ends it", SessionState{State: StateIdle}, "SessionEnd", HookInput{}, StateEnded, "", "", true},
		{"ended stays ended for a stray stop", SessionState{State: StateEnded}, "Stop", HookInput{}, StateEnded, "", "", false},
		{"ended stays ended for a late tool result", SessionState{State: StateEnded}, "PostToolUse", HookInput{}, StateEnded, "", "", false},
		{"a prompt after an end shows the session is alive", SessionState{State: StateEnded}, "UserPromptSubmit", HookInput{}, StateBusy, "", "", true},
		{"a start after an end shows the session is alive", SessionState{State: StateEnded}, "SessionStart", HookInput{}, StateIdle, "", "", true},
		{"unknown event", SessionState{State: StateIdle}, "PreToolUse", HookInput{}, StateIdle, "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := c.from
			if got := ApplyHook(&s, c.event, c.in, t0); got != c.write {
				t.Errorf("write = %v, want %v", got, c.write)
			}
			if s.State != c.want || s.Reason != c.reason || s.LastError != c.lastEr {
				t.Errorf("got %s/%q/%q, want %s/%q/%q", s.State, s.Reason, s.LastError, c.want, c.reason, c.lastEr)
			}
		})
	}
}

func TestApplyHookSetsSinceOnlyWhenTheStateChanges(t *testing.T) {
	s := SessionState{State: StateIdle, Since: t0.Add(-time.Hour)}
	ApplyHook(&s, "Stop", HookInput{}, t0)
	if !s.Since.Equal(t0.Add(-time.Hour)) {
		t.Errorf("since moved to %v without a change", s.Since)
	}
	ApplyHook(&s, "UserPromptSubmit", HookInput{}, t0)
	if !s.Since.Equal(t0) {
		t.Errorf("since = %v, want %v", s.Since, t0)
	}
}

func TestSessionStartKeepsTheModelWhenThePayloadHasNone(t *testing.T) {
	s := SessionState{State: StateIdle, Model: "claude-opus-5", Cwd: "/old"}
	ApplyHook(&s, "SessionStart", HookInput{Cwd: "/new"}, t0)
	if s.Model != "claude-opus-5" || s.Cwd != "/new" {
		t.Errorf("model %q cwd %q", s.Model, s.Cwd)
	}
}

func TestSessionStartForACompactionDoesNotTouchTheState(t *testing.T) {
	s := SessionState{State: StateBusy, Since: t0.Add(-time.Minute)}
	if ApplyHook(&s, "SessionStart", HookInput{Source: "compact"}, t0) || s.State != StateBusy {
		t.Errorf("a compaction in the middle of a reply made it %s", s.State)
	}
}

func TestANotificationRightAfterActivityIsLate(t *testing.T) {
	s := SessionState{State: StateBusy, Since: t0.Add(-time.Hour)}
	if ApplyHook(&s, "Notification", HookInput{NotificationType: "permission_prompt", RecentActivity: true}, t0) || s.State != StateBusy {
		t.Errorf("a prompt that was already answered made it %s", s.State)
	}
}
