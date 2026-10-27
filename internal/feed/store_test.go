package feed

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newStore(t *testing.T) Store { return Store{Dir: filepath.Join(t.TempDir(), "feed")} }

const h1 = "0123456789abcdef"

func TestUpdateSessionCreatesStartingThenWritesWhatFnChanged(t *testing.T) {
	s := newStore(t)
	err := s.UpdateSession(h1, "work", t0, func(x *SessionState) bool {
		if x.State != StateStarting || x.Handle != h1 || x.Account != "work" {
			t.Errorf("fresh state %+v", x)
		}
		x.Model = "m"
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s.Session(h1)
	if !ok || got.Model != "m" || !got.UpdatedAt.Equal(t0) || !got.Since.Equal(t0) {
		t.Errorf("%+v %v", got, ok)
	}
}

func TestUpdateSessionWritesNothingWhenFnSaysNoChange(t *testing.T) {
	s := newStore(t)
	if err := s.UpdateSession(h1, "work", t0, func(*SessionState) bool { return false }); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Session(h1); ok {
		t.Error("a file was written")
	}
}

func TestSessionToleratesMissingAndCorruptFilesAndBadHandles(t *testing.T) {
	s := newStore(t)
	_ = os.MkdirAll(filepath.Join(s.Dir, "sessions"), 0o700)
	_ = os.WriteFile(filepath.Join(s.Dir, "sessions", h1+".json"), []byte("{oops"), 0o600)
	if _, ok := s.Session(h1); ok {
		t.Error("corrupt file read as a session")
	}
	if _, ok := s.Session("../x"); ok {
		t.Error("bad handle accepted")
	}
	if got, err := s.Sessions(); err != nil || len(got) != 0 {
		t.Errorf("%v %v", got, err)
	}
	if err := s.UpdateSession("../x", "a", t0, func(*SessionState) bool { return true }); err == nil {
		t.Error("bad handle written")
	}
}

func TestSessionsListsEverythingReadable(t *testing.T) {
	s := newStore(t)
	for _, h := range []string{"bbbbbbbbbbbbbbbb", "aaaaaaaaaaaaaaaa"} {
		_ = s.UpdateSession(h, "work", t0, func(*SessionState) bool { return true })
	}
	got, err := s.Sessions()
	if err != nil || len(got) != 2 || got[0].Handle != "aaaaaaaaaaaaaaaa" {
		t.Errorf("%+v %v", got, err)
	}
}

func TestPruneRemovesSessionsThatAreNotLiveExceptRecentlyEnded(t *testing.T) {
	s := newStore(t)
	mk := func(h string, st State, at time.Time) {
		_ = s.UpdateSession(h, "w", at, func(x *SessionState) bool { x.State = st; return true })
	}
	live, gone, endedNew, endedOld := "1111111111111111", "2222222222222222", "3333333333333333", "4444444444444444"
	mk(live, StateBusy, t0)
	mk(gone, StateBusy, t0)
	mk(endedNew, StateEnded, t0.Add(-5*time.Minute))
	mk(endedOld, StateEnded, t0.Add(-11*time.Minute))

	n, err := s.Prune(t0, func(h string) bool { return h == live })

	if err != nil || n != 2 {
		t.Fatalf("pruned %d, %v", n, err)
	}
	for h, want := range map[string]bool{live: true, gone: false, endedNew: true, endedOld: false} {
		if _, ok := s.Session(h); ok != want {
			t.Errorf("%s present = %v, want %v", h, ok, want)
		}
	}
}

func TestRemoveSession(t *testing.T) {
	s := newStore(t)
	_ = s.UpdateSession(h1, "w", t0, func(*SessionState) bool { return true })
	if err := s.RemoveSession(h1); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Session(h1); ok {
		t.Error("still there")
	}
	if err := s.RemoveSession(h1); err != nil {
		t.Error(err)
	}
}

func TestActivityIsRememberedPerSessionWithoutTheLock(t *testing.T) {
	s := newStore(t)
	if !s.Activity(h1).IsZero() {
		t.Error("activity of a session nobody saw")
	}
	if err := s.TouchActivity(h1, t0); err != nil {
		t.Fatal(err)
	}
	if got := s.Activity(h1); !got.Equal(t0) {
		t.Errorf("got %v", got)
	}
	if err := s.TouchActivity("../x", t0); err == nil {
		t.Error("bad handle written")
	}
}
