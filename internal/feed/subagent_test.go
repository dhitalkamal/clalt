package feed

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func agentTool(desc, typ string) HookInput {
	return HookInput{ToolName: "Agent", ToolInput: json.RawMessage(`{"description":` + quote(desc) + `,"subagent_type":` + quote(typ) + `,"prompt":"SECRET"}`)}
}

func quote(s string) string { b, _ := json.Marshal(s); return string(b) }

func busy() *SessionState { return &SessionState{State: StateBusy} }

func TestSubagentStartThenStop(t *testing.T) {
	s := busy()
	if !ApplyHook(s, "SubagentStart", HookInput{AgentID: "a1", AgentType: "Explore"}, t0) {
		t.Fatal("start changed nothing")
	}
	if len(s.Subagents) != 1 || s.Subagents[0].State != "running" || !s.Subagents[0].StartedAt.Equal(t0) || s.Subagents[0].EndedAt != nil {
		t.Fatalf("%+v", s.Subagents)
	}
	// a repeated start updates by id instead of adding
	ApplyHook(s, "SubagentStart", HookInput{AgentID: "a1", AgentType: "Explore"}, t0.Add(time.Second))
	if len(s.Subagents) != 1 || !s.Subagents[0].StartedAt.Equal(t0) {
		t.Fatalf("%+v", s.Subagents)
	}
	t1 := t0.Add(time.Minute)
	ApplyHook(s, "SubagentStop", HookInput{AgentID: "a1", AgentType: "Explore"}, t1)
	got := s.Subagents[0]
	if got.State != "done" || got.EndedAt == nil || !got.EndedAt.Equal(t1) {
		t.Fatalf("%+v", got)
	}
	if s.State != StateBusy {
		t.Errorf("state moved to %s", s.State)
	}
}

func TestSubagentStopWithoutStartMakesADoneRecord(t *testing.T) {
	s := busy()
	ApplyHook(s, "SubagentStop", HookInput{AgentID: "a9", AgentType: "Plan"}, t0)
	if len(s.Subagents) != 1 || s.Subagents[0].State != "done" || s.Subagents[0].EndedAt == nil || s.Subagents[0].Type != "Plan" {
		t.Fatalf("%+v", s.Subagents)
	}
}

func TestSubagentWithoutAnIdIsIgnored(t *testing.T) {
	s := busy()
	if ApplyHook(s, "SubagentStart", HookInput{AgentType: "Plan"}, t0) || len(s.Subagents) != 0 {
		t.Error("recorded")
	}
}

func TestLabelsMatchFIFOByTypeForParallelSubagents(t *testing.T) {
	s := busy()
	ApplyHook(s, "PreToolUse", agentTool("first", "Explore"), t0)
	ApplyHook(s, "PreToolUse", agentTool("second", "Explore"), t0.Add(time.Second))
	ApplyHook(s, "PreToolUse", agentTool("other", "Plan"), t0.Add(2*time.Second))
	ApplyHook(s, "SubagentStart", HookInput{AgentID: "a", AgentType: "Explore"}, t0.Add(3*time.Second))
	ApplyHook(s, "SubagentStart", HookInput{AgentID: "b", AgentType: "Explore"}, t0.Add(3*time.Second))
	ApplyHook(s, "SubagentStart", HookInput{AgentID: "c", AgentType: "Plan"}, t0.Add(3*time.Second))
	ApplyHook(s, "SubagentStart", HookInput{AgentID: "d", AgentType: "Explore"}, t0.Add(3*time.Second))
	labels := ""
	for _, a := range s.Subagents {
		labels += a.ID + "=" + a.Label + ";"
	}
	if labels != "a=first;b=second;c=other;d=;" {
		t.Errorf("labels %s", labels)
	}
	if len(s.PendingAgents) != 0 {
		t.Errorf("pending left: %+v", s.PendingAgents)
	}
}

func TestPromptIsNeverStored(t *testing.T) {
	s := busy()
	ApplyHook(s, "PreToolUse", agentTool("look", "Explore"), t0)
	ApplyHook(s, "SubagentStart", HookInput{AgentID: "a", AgentType: "Explore"}, t0)
	raw, _ := json.Marshal(s)
	if strings.Contains(string(raw), "SECRET") {
		t.Errorf("prompt stored: %s", raw)
	}
}

func TestPreToolUseOfAnotherToolIsIgnored(t *testing.T) {
	s := busy()
	in := agentTool("x", "Explore")
	in.ToolName = "Bash"
	if ApplyHook(s, "PreToolUse", in, t0) || len(s.PendingAgents) != 0 {
		t.Error("bash recorded")
	}
	in.ToolName = "Task"
	if !ApplyHook(s, "PreToolUse", in, t0) || len(s.PendingAgents) != 1 {
		t.Error("task alias not recorded")
	}
}

func TestLabelIsCleanedAndCapped(t *testing.T) {
	s := busy()
	dirty := "fix\x1b[31m bug\u202e\u200b\nnow " + strings.Repeat("x", 200)
	ApplyHook(s, "PreToolUse", agentTool(dirty, "Explore"), t0)
	ApplyHook(s, "SubagentStart", HookInput{AgentID: "a", AgentType: "Explore"}, t0)
	l := s.Subagents[0].Label
	if len([]rune(l)) != 80 || !strings.HasPrefix(l, "fix[31m bug now xxx") {
		t.Errorf("label %q", l)
	}
	for _, r := range l {
		if r < 0x20 || r == 0x7f || r == '\u202e' || r == '\u200b' {
			t.Errorf("control char %U", r)
		}
	}
}

func TestSubagentListIsCappedDroppingOldestDoneFirst(t *testing.T) {
	s := busy()
	ApplyHook(s, "SubagentStart", HookInput{AgentID: "run0", AgentType: "t"}, t0)
	for i := 1; i <= 60; i++ {
		id := "d" + string(rune('A'+i/26)) + string(rune('a'+i%26))
		ApplyHook(s, "SubagentStart", HookInput{AgentID: id, AgentType: "t"}, t0.Add(time.Duration(i)*time.Second))
		ApplyHook(s, "SubagentStop", HookInput{AgentID: id, AgentType: "t"}, t0.Add(time.Duration(i)*time.Second))
	}
	if len(s.Subagents) != 50 {
		t.Fatalf("%d records", len(s.Subagents))
	}
	if s.Subagents[0].ID != "run0" || s.Subagents[0].State != "running" {
		t.Errorf("the running one was dropped: %+v", s.Subagents[0])
	}
}

func TestSessionEndClosesRunningSubagents(t *testing.T) {
	for _, reason := range []string{"clear", "other"} {
		s := busy()
		ApplyHook(s, "SubagentStart", HookInput{AgentID: "a", AgentType: "t"}, t0)
		ApplyHook(s, "SessionEnd", HookInput{Reason: reason}, t0.Add(time.Minute))
		if s.Subagents[0].State != "done" || s.Subagents[0].EndedAt == nil {
			t.Errorf("%s: %+v", reason, s.Subagents[0])
		}
	}
}

func TestPendingLabelsAreCapped(t *testing.T) {
	s := busy()
	for i := 0; i < 40; i++ {
		ApplyHook(s, "PreToolUse", agentTool("l", "t"), t0)
	}
	if len(s.PendingAgents) > 20 {
		t.Errorf("%d pending", len(s.PendingAgents))
	}
}

func TestStalePendingLabelIsNotTaken(t *testing.T) {
	s := busy()
	ApplyHook(s, "PreToolUse", agentTool("never started", "Explore"), t0)
	ApplyHook(s, "SubagentStart", HookInput{AgentID: "a", AgentType: "Explore"}, t0.Add(11*time.Minute))
	if s.Subagents[0].Label != "" {
		t.Errorf("label %q", s.Subagents[0].Label)
	}
	if len(s.PendingAgents) != 0 {
		t.Errorf("stale entry kept: %+v", s.PendingAgents)
	}
}

func TestStopAndSessionEndClearPendingLabels(t *testing.T) {
	for _, ev := range []string{"Stop", "SessionEnd"} {
		s := busy()
		ApplyHook(s, "PreToolUse", agentTool("denied", "Explore"), t0)
		ApplyHook(s, ev, HookInput{Reason: "other"}, t0.Add(time.Second))
		if len(s.PendingAgents) != 0 {
			t.Errorf("%s left %+v", ev, s.PendingAgents)
		}
	}
}
