package feed

import (
	"strings"
	"testing"
	"time"
)

func TestExtHandleKeepsAClaudeSessionIdAndRefusesAnythingThatIsAPath(t *testing.T) {
	id := "3f2c9a7e-1b4d-4c8e-9a52-0d6f1e8b7a31"
	if h, ok := ExtHandle(id); !ok || h != "ext-"+id || !IsExt(h) {
		t.Errorf("%q %v", h, ok)
	}
	for _, bad := range []string{"", "../x", "a/b", `a\b`, "a b", "a.b", strings.Repeat("a", 61), "id\n"} {
		if h, ok := ExtHandle(bad); ok {
			t.Errorf("%q accepted as %q", bad, h)
		}
	}
	if IsExt("0123456789abcdef") || IsExt("ext-") || IsExt("ext-../x") {
		t.Error("IsExt too lenient")
	}
}

func TestTheStoreKeepsSessionsKeyedByAnExtHandle(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	h, _ := ExtHandle("abc-123")
	now := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)

	err := s.UpdateSession(h, "work", now, func(st *SessionState) bool { return ApplyHook(st, "UserPromptSubmit", HookInput{}, now) })

	if err != nil {
		t.Fatal(err)
	}
	st, ok := s.Session(h)
	if !ok || st.Account != "work" || st.State != StateBusy {
		t.Errorf("%+v %v", st, ok)
	}
	if all, _ := s.Sessions(); len(all) != 1 || all[0].Handle != h {
		t.Errorf("%+v", all)
	}
	if err := s.TouchActivity(h, now); err != nil {
		t.Error(err)
	}
	if err := s.RemoveSession(h); err != nil {
		t.Error(err)
	}
	if _, ok := s.Session(h); ok {
		t.Error("still there")
	}
}
