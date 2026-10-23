package account

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestResolveLabel(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "Robin")
	work := mustCreate(t, s, "work")
	if err := s.Relabel(work.Name, "office"); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		label string
		id    string // empty means not found
	}{
		{"Robin", "Robin"},
		{"robin", "Robin"},
		{"ROBIN", "Robin"},
		{"office", "work"},
		{"OFFICE", "work"},
		// the id of a relabeled account is not a handle any more
		{"work", ""},
		{"nobody", ""},
	}
	for _, c := range cases {
		got, found, err := s.ResolveLabel(c.label)
		if err != nil {
			t.Errorf("ResolveLabel(%q): %v", c.label, err)
			continue
		}
		if found != (c.id != "") || got.Name != c.id {
			t.Errorf("ResolveLabel(%q) = %q, found %v, want %q", c.label, got.Name, found, c.id)
		}
	}
}

func TestResolveLabelFindsLegacyAccountsAndFlagsHiddenOnes(t *testing.T) {
	s := newMetaStore(t)
	addLegacy(t, s, "work", Meta{Email: "w@example.com"})
	gone := mustCreate(t, s, "gone")
	if err := s.Hide(gone.Name); err != nil {
		t.Fatal(err)
	}

	legacy, found, err := s.ResolveLabel("WORK")
	if err != nil || !found || legacy.Name != "work" || legacy.Meta.Hidden {
		t.Errorf("legacy: %+v found=%v err=%v", legacy, found, err)
	}
	hidden, found, err := s.ResolveLabel("gone")
	if err != nil || !found || !hidden.Meta.Hidden {
		t.Errorf("hidden: %+v found=%v err=%v", hidden, found, err)
	}
}

func TestResolveLabelPrefersTheExactSpellingAndRefusesAnAmbiguousOne(t *testing.T) {
	s := newMetaStore(t)
	// only reachable through a state file from before names were checked
	addLegacy(t, s, "Robin", Meta{})
	addLegacy(t, s, "robin", Meta{})

	got, found, err := s.ResolveLabel("robin")
	if err != nil || !found || got.Name != "robin" {
		t.Errorf("exact: %+v found=%v err=%v", got, found, err)
	}
	if _, _, err := s.ResolveLabel("ROBIN"); err == nil {
		t.Error("ResolveLabel(ROBIN) picked one of two accounts")
	}
}

func TestSetIdentityRecordsWhoSignedIn(t *testing.T) {
	s := newMetaStore(t)
	acc := mustCreate(t, s, "robin")
	at := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)

	err := s.SetIdentity(acc.Name, Identity{Email: "k@example.com", Org: "Example Org", Plan: "max", SignedInAt: at})

	if err != nil {
		t.Fatal(err)
	}
	m := stored(t, s, "robin").Meta
	if m.Email != "k@example.com" || m.Org != "Example Org" || m.Plan != "max" || !m.SignedInAt.Equal(at) {
		t.Errorf("meta = %+v", m)
	}
	if m.Label != "robin" || !m.AddedAt.Equal(fixedNow) {
		t.Errorf("SetIdentity changed the rest of the meta: %+v", m)
	}
}

func TestSetIdentityStampsTheStoreClockWhenNoTimeIsGiven(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")
	if err := s.SetIdentity("robin", Identity{Email: "k@example.com"}); err != nil {
		t.Fatal(err)
	}
	if got := stored(t, s, "robin").Meta.SignedInAt; !got.Equal(fixedNow) {
		t.Errorf("SignedInAt = %v, want %v", got, fixedNow)
	}
}

func TestSetIdentityReplacesTheWholeIdentity(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")
	if err := s.SetIdentity("robin", Identity{Email: "k@example.com", Org: "Acme", Plan: "max"}); err != nil {
		t.Fatal(err)
	}
	// a personal plan has no org: the old one must not linger
	if err := s.SetIdentity("robin", Identity{Email: "k@example.com", Plan: "pro"}); err != nil {
		t.Fatal(err)
	}
	if m := stored(t, s, "robin").Meta; m.Org != "" || m.Plan != "pro" {
		t.Errorf("meta = %+v", m)
	}
}

func TestSetIdentityNeedsAnEmailAndAKnownAccount(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")
	if err := s.SetIdentity("robin", Identity{Org: "Acme"}); err == nil {
		t.Error("SetIdentity accepted an identity without an email")
	}
	if err := s.SetIdentity("nobody", Identity{Email: "x@example.com"}); err == nil {
		t.Error("SetIdentity accepted an unknown account")
	}
}

func TestClearSignInKeepsTheEmailSoTheNextLoginIsStillChecked(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")
	if err := s.SetIdentity("robin", Identity{Email: "k@example.com", Org: "Acme", Plan: "max"}); err != nil {
		t.Fatal(err)
	}

	if err := s.ClearSignIn("robin"); err != nil {
		t.Fatal(err)
	}

	m := stored(t, s, "robin").Meta
	if m.Email != "k@example.com" || m.Org != "" || m.Plan != "" || !m.SignedInAt.IsZero() {
		t.Errorf("meta = %+v", m)
	}
	if m.Label != "robin" || m.Hidden {
		t.Errorf("ClearSignIn changed more than the sign-in: %+v", m)
	}
	if err := s.ClearSignIn("nobody"); err == nil {
		t.Error("ClearSignIn accepted an unknown account")
	}
}

func TestHideDropsTheIdentityAndKeepsTheAccount(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")
	if err := s.SetIdentity("robin", Identity{Email: "k@example.com", Org: "Acme", Plan: "max"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetBudget("robin", 5, 20); err != nil {
		t.Fatal(err)
	}
	if err := s.Touch("robin"); err != nil {
		t.Fatal(err)
	}

	if err := s.Hide("robin"); err != nil {
		t.Fatal(err)
	}

	m := stored(t, s, "robin").Meta
	if !m.Hidden || m.Email != "" || m.Org != "" || m.Plan != "" || !m.SignedInAt.IsZero() {
		t.Errorf("meta = %+v", m)
	}
	// history, the label and the folder id survive a remove
	if m.Label != "robin" || !m.AddedAt.Equal(fixedNow) || !m.LastUsedAt.Equal(fixedNow) || m.DailyBudget != 5 || m.WeeklyBudget != 20 {
		t.Errorf("Hide dropped more than the identity: %+v", m)
	}
	if err := s.Hide("nobody"); err == nil {
		t.Error("Hide accepted an unknown account")
	}
}

func TestUnhideBringsTheAccountBackWithoutAnIdentity(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")
	if err := s.Hide("robin"); err != nil {
		t.Fatal(err)
	}

	if err := s.Unhide("robin"); err != nil {
		t.Fatal(err)
	}

	if m := stored(t, s, "robin").Meta; m.Hidden || m.Email != "" || m.Label != "robin" {
		t.Errorf("meta = %+v", m)
	}
	if err := s.Unhide("nobody"); err == nil {
		t.Error("Unhide accepted an unknown account")
	}
}

func TestRelabelChangesOnlyTheLabel(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")
	if err := s.SetIdentity("robin", Identity{Email: "k@example.com"}); err != nil {
		t.Fatal(err)
	}

	if err := s.Relabel("robin", "Home"); err != nil {
		t.Fatal(err)
	}

	got := stored(t, s, "robin")
	if got.Name != "robin" || got.Label() != "Home" || got.Meta.Email != "k@example.com" {
		t.Errorf("account = %+v", got)
	}
	// a case-only change and a no-op are fine for the account itself
	for _, label := range []string{"home", "home"} {
		if err := s.Relabel("robin", label); err != nil {
			t.Errorf("Relabel(%q): %v", label, err)
		}
	}
	if got := stored(t, s, "robin").Label(); got != "home" {
		t.Errorf("label = %q, want home", got)
	}
}

func TestRelabelIsValidatedAndUnique(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")
	mustCreate(t, s, "work")
	gone := mustCreate(t, s, "gone")
	if err := s.Hide(gone.Name); err != nil {
		t.Fatal(err)
	}
	addLegacy(t, s, "old", Meta{})

	for _, label := range []string{"", "../x", ".hidden", "has space", "work", "WORK", "gone", "old"} {
		if err := s.Relabel("robin", label); err == nil {
			t.Errorf("Relabel(%q) succeeded", label)
		}
	}
	if got := stored(t, s, "robin").Label(); got != "robin" {
		t.Errorf("a rejected relabel changed the label to %q", got)
	}
	if err := s.Relabel("nobody", "fresh"); err == nil {
		t.Error("Relabel accepted an unknown account")
	}
}

func TestTouchStampsLastUsedAt(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")
	later := fixedNow.Add(3 * time.Hour)
	s.now = func() time.Time { return later }

	if err := s.Touch("robin"); err != nil {
		t.Fatal(err)
	}

	m := stored(t, s, "robin").Meta
	if !m.LastUsedAt.Equal(later) || !m.AddedAt.Equal(fixedNow) || m.Label != "robin" {
		t.Errorf("meta = %+v", m)
	}
	if err := s.Touch("nobody"); err == nil {
		t.Error("Touch accepted an unknown account")
	}
}

func TestFailedMetadataCallsWriteNothing(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")
	before, err := os.ReadFile(s.statePath)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Touch("nobody")
	_ = s.Relabel("robin", "../x")
	_ = s.SetIdentity("robin", Identity{})
	_, _ = s.Create("ROBIN", noFolders)
	after, err := os.ReadFile(s.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Error("a failed call changed state.json")
	}
}
