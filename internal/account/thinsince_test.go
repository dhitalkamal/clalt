package account

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// the switch log stopped growing when accounts became folders. the store keeps
// the time of the first sign-in that worked in the new design, so that usage
// from then on is not charged to the account of the last old event.
func TestThinSinceIsUnsetUntilAnAccountSignsIn(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")

	got, err := s.ThinSince()

	if err != nil || !got.IsZero() {
		t.Errorf("ThinSince = %v, %v; want unset", got, err)
	}
	raw, err := os.ReadFile(s.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if topLevelKeys(t, raw)["thinSince"] {
		t.Errorf("the key is in a file that has no sign-in yet:\n%s", raw)
	}
}

func TestTheFirstSignInRecordsThinSinceAndNoLaterOneChangesIt(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")
	mustCreate(t, s, "work")
	first := time.Date(2026, 10, 22, 8, 0, 0, 0, time.UTC)
	later := first.Add(48 * time.Hour)

	if err := s.SetIdentity("robin", Identity{Email: "k@example.com", SignedInAt: first}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetIdentity("work", Identity{Email: "w@example.com", SignedInAt: later}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetIdentity("robin", Identity{Email: "k@example.com", SignedInAt: later}); err != nil {
		t.Fatal(err)
	}
	if err := s.ClearSignIn("robin"); err != nil {
		t.Fatal(err)
	}
	if err := s.Hide("work"); err != nil {
		t.Fatal(err)
	}

	got, err := s.ThinSince()

	if err != nil || !got.Equal(first) {
		t.Errorf("ThinSince = %v, %v; want %v, the first sign-in", got, err, first)
	}
}

func TestThinSinceTakesTheStoreClockWhenTheSignInHasNoTime(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")

	if err := s.SetIdentity("robin", Identity{Email: "k@example.com"}); err != nil {
		t.Fatal(err)
	}

	if got, err := s.ThinSince(); err != nil || !got.Equal(fixedNow) {
		t.Errorf("ThinSince = %v, %v; want %v", got, err, fixedNow)
	}
}

// a sign-in that is refused records nothing, and so does not start the clock
func TestASignInThatFailsDoesNotStartThinSince(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")

	if err := s.SetIdentity("robin", Identity{}); err == nil {
		t.Fatal("an identity without an email was accepted")
	}
	if err := s.SetIdentity("nobody", Identity{Email: "x@example.com"}); err == nil {
		t.Fatal("an unknown account was accepted")
	}

	if got, _ := s.ThinSince(); !got.IsZero() {
		t.Errorf("ThinSince = %v after sign-ins that did not work", got)
	}
}

// the file is still one that v0.1.0 reads: the key is new and the old binary
// ignores what it does not know
func TestThinSinceIsAnAdditiveKey(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")
	if err := s.SetIdentity("robin", Identity{Email: "k@example.com", SignedInAt: fixedNow}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(s.statePath)
	if err != nil {
		t.Fatal(err)
	}

	if !topLevelKeys(t, raw)["thinSince"] {
		t.Errorf("no thinSince in the file:\n%s", raw)
	}
	var old v010State
	if err := json.Unmarshal(raw, &old); err != nil {
		t.Errorf("v0.1.0 cannot read the file: %v", err)
	}
}

func TestAnOldStateFileHasNoThinSinceAndKeepsWorking(t *testing.T) {
	s := newStore(t)
	if err := os.WriteFile(s.statePath, []byte(v010File), 0o600); err != nil {
		t.Fatal(err)
	}

	if got, err := s.ThinSince(); err != nil || !got.IsZero() {
		t.Errorf("ThinSince = %v, %v; want unset for a file from v0.1.0", got, err)
	}
}

func topLevelKeys(t *testing.T, raw []byte) map[string]bool {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{}
	for k := range top {
		keys[k] = true
	}
	return keys
}
