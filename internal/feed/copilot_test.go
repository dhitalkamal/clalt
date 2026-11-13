package feed

import (
	"strings"
	"testing"
	"time"
)

func TestCopilotHandleKeepsASessionIdAndRefusesAnythingThatIsAPath(t *testing.T) {
	id := "3f2c9a7e-1b4d-4c8e-9a52-0d6f1e8b7a31"
	if h, ok := CopilotHandle(id); !ok || h != "cp-"+id || !IsCopilot(h) {
		t.Errorf("%q %v", h, ok)
	}
	for _, bad := range []string{"", "../x", "a/b", `a\b`, "a b", "a.b", strings.Repeat("a", 61), "id\n"} {
		if h, ok := CopilotHandle(bad); ok {
			t.Errorf("%q accepted as %q", bad, h)
		}
	}
	if IsCopilot("0123456789abcdef") || IsCopilot("cp-") || IsCopilot("cp-../x") {
		t.Error("IsCopilot too lenient")
	}
}

func TestACopilotHandleIsAValidKeyButNotAnExtHandle(t *testing.T) {
	h, _ := CopilotHandle("abc-123")
	if !ValidKey(h) {
		t.Error("ValidKey refuses it")
	}
	if IsExt(h) {
		t.Error("cp handle read as ext")
	}
	if eh, _ := ExtHandle("abc-123"); IsCopilot(eh) {
		t.Error("ext handle read as copilot")
	}
}

func TestTheStoreKeepsACopilotSessionAndItsTool(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	h, _ := CopilotHandle("abc-123")
	now := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)

	err := s.UpdateSession(h, "~copilot", now, func(st *SessionState) bool {
		st.Tool = "copilot"
		return ApplyHook(st, "UserPromptSubmit", HookInput{}, now)
	})

	if err != nil {
		t.Fatal(err)
	}
	st, ok := s.Session(h)
	if !ok || st.Tool != "copilot" || st.State != StateBusy {
		t.Errorf("%+v %v", st, ok)
	}
}
