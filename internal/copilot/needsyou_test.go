package copilot

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
)

func req(id string) string {
	return ev("permission.requested", map[string]any{
		"requestId":         id,
		"permissionRequest": map[string]any{"command": marker},
		"promptRequest":     map[string]any{"text": marker},
	})
}

func done(id string) string {
	return ev("permission.completed", map[string]any{"requestId": id, "result": map[string]any{"kind": "approved"}, "decisionSource": "human_response"})
}

// one returns the only listed session after writing the lines.
func one(t *testing.T, lines ...string) Session {
	t.Helper()
	f := newFixture(t)
	f.session(id1, time.Minute, append([]string{start(id1, "/p/a")}, lines...)...)
	got := f.list()
	if len(got) != 1 {
		t.Fatalf("%+v", got)
	}
	return got[0]
}

func wantState(t *testing.T, s Session, st feed.State, reason string) {
	t.Helper()
	if s.State != st || s.Reason != reason {
		t.Errorf("state %q reason %q, want %q %q", s.State, s.Reason, st, reason)
	}
}

func TestAnOpenRequestNeedsYou(t *testing.T) {
	wantState(t, one(t, prompt(), req("r1")), feed.StateNeedsYou, "permission_prompt")
}

func TestACompletedRequestGoesBackToBusy(t *testing.T) {
	wantState(t, one(t, prompt(), req("r1"), done("r1")), feed.StateBusy, "")
}

func TestStillNeedsYouWhileAnyOfThreeIsOpen(t *testing.T) {
	wantState(t, one(t, prompt(), req("a"), req("b"), req("c"), done("a"), done("c")), feed.StateNeedsYou, "permission_prompt")
}

func TestBusyEventsKeepNeedsYou(t *testing.T) {
	wantState(t, one(t, prompt(), req("a"), ev("tool.execution_start", map[string]any{"toolName": marker}), ev("tool.execution_complete", nil)), feed.StateNeedsYou, "permission_prompt")
}

func TestToolStartIsBusy(t *testing.T) {
	wantState(t, one(t, ev("assistant.turn_end", nil), ev("tool.execution_start", nil)), feed.StateBusy, "")
}

func TestAbortClearsAndGoesIdle(t *testing.T) {
	wantState(t, one(t, prompt(), req("a"), ev("abort", map[string]any{"reason": "user_initiated"})), feed.StateIdle, "")
}

func TestTurnEndClearsAndGoesIdle(t *testing.T) {
	s := one(t, prompt(), req("a"), ev("assistant.turn_end", nil))
	wantState(t, s, feed.StateIdle, "")
	// a late completed for the cleared id must not bring anything back
	wantState(t, one(t, req("a"), ev("assistant.turn_end", nil), done("a")), feed.StateIdle, "")
}

func TestAnUnknownCompletedIdIsIgnored(t *testing.T) {
	wantState(t, one(t, prompt(), req("a"), done("zzz")), feed.StateNeedsYou, "permission_prompt")
	wantState(t, one(t, prompt(), done("zzz")), feed.StateBusy, "")
}

func TestTheOpenSetIsCapped(t *testing.T) {
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, req(fmt.Sprintf("id-%d", i)))
	}
	lines = append(lines, req(strings.Repeat("x", 5000)))
	f := newFixture(t)
	f.session(id1, time.Minute, append([]string{start(id1, "/p/a")}, lines...)...)
	f.list()
	e := f.reader().cache[id1]
	if n := len(e.sum.open); n > maxOpen {
		t.Errorf("open set holds %d ids", n)
	}
	for id := range e.sum.open {
		if len(id) > maxIDLen {
			t.Errorf("id length %d", len(id))
		}
	}
	// the capped set still reports needs-you
	wantState(t, f.list()[0], feed.StateNeedsYou, "permission_prompt")
}

func TestTheOpenSetSurvivesAnIncrementalRead(t *testing.T) {
	f := newFixture(t)
	f.session(id1, time.Minute, start(id1, "/p/a"), prompt(), req("a"), req("b"))
	wantState(t, f.list()[0], feed.StateNeedsYou, "permission_prompt")
	appendRaw(t, f, id1, done("a")+"\n")
	wantState(t, f.list()[0], feed.StateNeedsYou, "permission_prompt")
	appendRaw(t, f, id1, done("b")+"\n")
	wantState(t, f.list()[0], feed.StateBusy, "")
}

func TestPermissionTextNeverComesOut(t *testing.T) {
	s := one(t, prompt(), req("a"))
	if got := fmt.Sprintf("%+v", s); strings.Contains(got, marker) {
		t.Errorf("marker leaked: %s", got)
	}
}

func modelEv(name, source string) string {
	return ev("session.model_change", map[string]any{"newModel": name, "source": source})
}

func chosen(name string) string {
	return ev("session.auto_mode_resolved", map[string]any{"chosenModel": name, "selectionReason": marker, "candidateModels": []string{marker}})
}

func TestAgentModelChangesAreIgnored(t *testing.T) {
	s := one(t, modelEv("gpt-5", "automatic"), modelEv("copilot-search-a", "agent"))
	if s.Model != "gpt-5" {
		t.Errorf("model %q", s.Model)
	}
}

func TestAutoShowsTheChosenModel(t *testing.T) {
	s := one(t, modelEv("auto", "automatic"), chosen("claude-sonnet-4.5"), modelEv("helper", "agent"))
	if s.Model != "claude-sonnet-4.5" {
		t.Errorf("model %q", s.Model)
	}
}

func TestAutoWithNoChoiceYetShowsAuto(t *testing.T) {
	if s := one(t, modelEv("Auto", "automatic")); s.Model != "Auto" {
		t.Errorf("model %q", s.Model)
	}
}

func TestALaterConcreteModelBeatsAnOlderChoice(t *testing.T) {
	s := one(t, modelEv("auto", "automatic"), chosen("claude-sonnet-4.5"), modelEv("gpt-5", "user"))
	if s.Model != "gpt-5" {
		t.Errorf("model %q", s.Model)
	}
	// switching back to auto waits for a fresh choice
	s = one(t, modelEv("auto", "automatic"), chosen("claude-sonnet-4.5"), modelEv("gpt-5", "user"), modelEv("auto", "user"))
	if s.Model != "auto" {
		t.Errorf("model %q", s.Model)
	}
}
