package cli

import (
	"testing"

	"github.com/dhitalkamal/clalt/internal/feed"
)

func TestHookRecordsSubagentsSilently(t *testing.T) {
	e := newEnv(t)
	h := liveHandle(t)
	e.hook("SessionStart", `{"session_id":"c1","cwd":"/w"}`)

	for _, step := range [][2]string{
		{"PreToolUse", `{"session_id":"c1","tool_name":"Agent","tool_input":{"description":"scan repo","subagent_type":"Explore","prompt":"TOPSECRET"}}`},
		{"SubagentStart", `{"session_id":"c1","agent_id":"a1","agent_type":"Explore"}`},
		{"SubagentStop", `{"session_id":"c1","agent_id":"a1","agent_type":"Explore","agent_transcript_path":"/t","last_assistant_message":"TOPSECRET"}`},
	} {
		e.out.Reset()
		if code := e.hook(step[0], step[1]); code != 0 || e.out.Len() != 0 || e.errOut.Len() != 0 {
			t.Fatalf("%s: exit %d out %q err %q", step[0], code, e.out.String(), e.errOut.String())
		}
	}
	st, _ := feedStore().Session(h)
	if len(st.Subagents) != 1 || st.Subagents[0].Label != "scan repo" || st.Subagents[0].State != "done" || st.Subagents[0].EndedAt == nil {
		t.Fatalf("%+v", st.Subagents)
	}
	if st.State != feed.StateIdle {
		t.Errorf("state %s", st.State)
	}
}

func TestHookSessionEndClosesRunningSubagents(t *testing.T) {
	e := newEnv(t)
	h := liveHandle(t)
	e.hook("SubagentStart", `{"session_id":"c1","agent_id":"a1","agent_type":"Plan"}`)
	e.hook("SessionEnd", `{"session_id":"c1","reason":"prompt_input_exit"}`)
	st, _ := feedStore().Session(h)
	if len(st.Subagents) != 1 || st.Subagents[0].State != "done" {
		t.Errorf("%+v", st.Subagents)
	}
}
