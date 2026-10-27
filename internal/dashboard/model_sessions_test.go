package dashboard

import (
	"strings"
	"testing"
)

// robin's sessions are s1 and s2. withSessions is a snapshot where robin runs
// the sessions with these ids, in this order.
func withSessions(ids ...string) Snapshot {
	snap := fiveSnapshot()
	var list []Session
	for _, id := range ids {
		list = append(list, Session{ID: id, State: StateWorking, Folder: "~/" + id, Model: "Opus 5.5"})
	}
	snap.Accounts[0].Sessions = list
	return snap
}

func onSecondSession(t *testing.T) (model, *fakeActions) {
	t.Helper()
	act := &fakeActions{}
	m := loadedModel(&fakeSource{snap: withSessions("s1", "s2")}, act, testOptions(), 100, 30)
	m = pressSettle(m, "2", "tab", "j")
	if m.sessID != "s2" || m.rowCur != 1 {
		t.Fatalf("setup: s2 is under the cursor, got %q at %d", m.sessID, m.rowCur)
	}
	return m, act
}

func TestTheSessionCursorStaysOnItsSessionWhenANewOneAppearsFirst(t *testing.T) {
	m, act := onSecondSession(t)
	m, _ = upd(m, loadedMsg{snap: withSessions("s0", "s1", "s2")})
	if m.sessID != "s2" || m.rowCur != 2 || !strings.Contains(Screen(m), "> working  ~/s2") {
		t.Errorf("the cursor follows s2 to row 2: %q at %d\n%s", m.sessID, m.rowCur, Screen(m))
	}
	pressSettle(m, "x", "y")
	if got := act.log(); len(got) != 1 || got[0] != "stop a1 s2" {
		t.Errorf("x stops the session that was selected, not the one that took its row: %v", got)
	}
}

func TestTheSessionCursorFollowsAReorder(t *testing.T) {
	m, _ := onSecondSession(t)
	m, _ = upd(m, loadedMsg{snap: withSessions("s2", "s1")})
	if m.sessID != "s2" || m.rowCur != 0 {
		t.Errorf("s2 moved to the top and the cursor with it: %q at %d", m.sessID, m.rowCur)
	}
}

func TestTheSessionCursorMovesToANeighbourWhenItsSessionEnds(t *testing.T) {
	m, act := onSecondSession(t)
	m, _ = upd(m, loadedMsg{snap: withSessions("s1")})
	if m.sessID != "s1" || m.rowCur != 0 {
		t.Fatalf("s2 ended, so the cursor lands on the one that is left: %q at %d", m.sessID, m.rowCur)
	}
	pressSettle(m, "x", "y")
	if got := act.log(); len(got) != 1 || got[0] != "stop a1 s1" {
		t.Errorf("the stop goes to what the screen shows: %v", got)
	}
	gone, _ := upd(m, loadedMsg{snap: withSessions()})
	if gone.sessID != "" || gone.rowCur != 0 {
		t.Errorf("no sessions, no selection: %q at %d", gone.sessID, gone.rowCur)
	}
}

func TestMovingTheCursorChoosesAnotherSession(t *testing.T) {
	m, _ := onSecondSession(t)
	m = pressSettle(m, "k")
	if m.sessID != "s1" || m.rowCur != 0 {
		t.Errorf("k selects s1: %q at %d", m.sessID, m.rowCur)
	}
	m = pressSettle(m, "j", "j", "j")
	if m.sessID != "s2" || m.rowCur != 1 {
		t.Errorf("j stops at the last session: %q at %d", m.sessID, m.rowCur)
	}
}

func TestTheSessionChoiceStartsOverWithAnotherAccountOrView(t *testing.T) {
	m, _ := onSecondSession(t)
	other := pressSettle(m, "tab", "j", "tab")
	if other.sessID != "" || other.rowCur != 0 {
		t.Errorf("another account has its own sessions: %q at %d", other.sessID, other.rowCur)
	}
	back := pressSettle(m, "1", "2")
	if back.sessID != "s1" || back.rowCur != 0 {
		t.Errorf("coming back to the view starts at the first session: %q at %d", back.sessID, back.rowCur)
	}
}

func TestAnAskedStopKeepsItsSessionWhileTheListRefreshes(t *testing.T) {
	m, act := onSecondSession(t)
	asked, _ := Press(m, "x")
	refreshed, _ := upd(asked.(model), loadedMsg{snap: withSessions("s0", "s1", "s2")})
	pressSettle(refreshed, "y")
	if got := act.log(); len(got) != 1 || got[0] != "stop a1 s2" {
		t.Errorf("the question named s2 and y answers it: %v", got)
	}
}
