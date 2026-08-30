package account

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/credential"
	"github.com/dhitalkamal/clalt/internal/keychain"
)

func newStore(t *testing.T) (*Store, *keychain.Fake) {
	t.Helper()
	dir := t.TempDir()
	kc := keychain.NewFake()
	s := New(kc, filepath.Join(dir, "state.json"), filepath.Join(dir, "switches.jsonl"))
	return s, kc
}

func mkCred(access string, exp int64) credential.Credential {
	return credential.Credential{AccessToken: access, RefreshToken: "r-" + access, ExpiresAt: exp, SubscriptionType: "max"}
}

func TestSaveAndGet(t *testing.T) {
	s, _ := newStore(t)
	c := mkCred("tok", 123)
	if err := s.Save("work", c, Meta{Email: "w@x.com", Plan: "max"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	gotC, gotMeta, err := s.Get("work")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if gotC.AccessToken != "tok" {
		t.Errorf("cred = %+v", gotC)
	}
	if gotMeta.Email != "w@x.com" || gotMeta.AddedAt.IsZero() {
		t.Errorf("meta = %+v", gotMeta)
	}
}

func TestGetMissing(t *testing.T) {
	s, _ := newStore(t)
	if _, _, err := s.Get("nope"); err == nil {
		t.Error("expected error for missing account")
	}
}

func TestListMarksActive(t *testing.T) {
	s, _ := newStore(t)
	_ = s.Save("a", mkCred("a", 1), Meta{})
	_ = s.Save("b", mkCred("b", 1), Meta{})
	if err := s.SetActive("b"); err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d", len(list))
	}
	for _, a := range list {
		if a.Name == "b" && !a.Active {
			t.Error("b should be active")
		}
		if a.Name == "a" && a.Active {
			t.Error("a should not be active")
		}
	}
}

func TestSavePreservesAddedAt(t *testing.T) {
	s, _ := newStore(t)
	fixed := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed }
	_ = s.Save("a", mkCred("a", 1), Meta{})
	// re-save later; AddedAt must not move
	s.now = func() time.Time { return fixed.Add(48 * time.Hour) }
	_ = s.Save("a", mkCred("a2", 2), Meta{Email: "new@x.com"})
	_, meta, _ := s.Get("a")
	if !meta.AddedAt.Equal(fixed) {
		t.Errorf("AddedAt moved to %v", meta.AddedAt)
	}
	if meta.Email != "new@x.com" {
		t.Errorf("email not updated: %q", meta.Email)
	}
}

func TestRemoveGuardsActive(t *testing.T) {
	s, _ := newStore(t)
	_ = s.Save("a", mkCred("a", 1), Meta{})
	_ = s.SetActive("a")
	if err := s.Remove("a"); err == nil {
		t.Error("removing the active account should error")
	}
	// non-active removes cleanly
	_ = s.Save("b", mkCred("b", 1), Meta{})
	if err := s.Remove("b"); err != nil {
		t.Errorf("Remove(b): %v", err)
	}
	if _, _, err := s.Get("b"); err == nil {
		t.Error("b should be gone")
	}
}

func TestRenameFixesPointer(t *testing.T) {
	s, _ := newStore(t)
	_ = s.Save("old", mkCred("tok", 1), Meta{Email: "e@x.com"})
	_ = s.SetActive("old")
	if err := s.Rename("old", "new"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if a, _ := s.Active(); a != "new" {
		t.Errorf("active pointer = %q, want new", a)
	}
	if _, _, err := s.Get("old"); err == nil {
		t.Error("old name should be gone")
	}
	c, meta, err := s.Get("new")
	if err != nil || c.AccessToken != "tok" || meta.Email != "e@x.com" {
		t.Errorf("new slot wrong: %+v %+v %v", c, meta, err)
	}
}
