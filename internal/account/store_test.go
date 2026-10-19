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

// TestSaveRejectsInvalidNewName proves a brand-new name is validated before
// anything is written: a malicious backup key like "../../../../tmp/x" must
// never reach the vault or state.json.
func TestSaveRejectsInvalidNewName(t *testing.T) {
	s, kc := newStore(t)
	if err := s.Save("../../../../tmp/x", mkCred("tok", 1), Meta{}); err == nil {
		t.Fatal("Save with a path-traversal name should error")
	}
	if _, _, err := s.Get("../../../../tmp/x"); err == nil {
		t.Error("nothing should have been written for a rejected name")
	}
	if names, _ := kc.List(KeychainService); len(names) != 0 {
		t.Errorf("keychain should be untouched, got %v", names)
	}
}

// TestSaveAllowsUpdatingExistingInvalidLegacyName proves accounts added before
// this validation existed are not bricked: re-saving (e.g. a token refresh)
// under the same, already-stored name must keep working even if that name
// would now fail ValidateName.
func TestSaveAllowsUpdatingExistingInvalidLegacyName(t *testing.T) {
	s, kc := newStore(t)
	// simulate a pre-existing legacy account with a now-invalid name, bypassing
	// Save's validation the way an old on-disk state.json would.
	legacy := "weird name!"
	if err := kc.Set(KeychainService, legacy, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	st, _ := s.load()
	st.Accounts[legacy] = Meta{}
	if err := s.save(st); err != nil {
		t.Fatal(err)
	}

	// re-saving (as a refresh would) must succeed, not brick the account
	if err := s.Save(legacy, mkCred("tok2", 2), Meta{Email: "e@x.com"}); err != nil {
		t.Fatalf("Save on existing legacy name should succeed, got %v", err)
	}
	_, meta, err := s.Get(legacy)
	if err != nil || meta.Email != "e@x.com" {
		t.Errorf("legacy account not updated: %+v %v", meta, err)
	}
}

// TestRenameRejectsInvalidNewName proves the destination name of a rename is
// validated (it becomes a new storage key / profile dir name).
func TestRenameRejectsInvalidNewName(t *testing.T) {
	s, _ := newStore(t)
	_ = s.Save("old", mkCred("tok", 1), Meta{})
	if err := s.Rename("old", "../evil"); err == nil {
		t.Fatal("Rename to a path-traversal name should error")
	}
	// nothing should have moved
	if _, _, err := s.Get("old"); err != nil {
		t.Errorf("old account should be untouched after a rejected rename: %v", err)
	}
}

// TestRenameFromInvalidLegacyNameSucceeds proves the only way out of an
// invalid legacy name (`clalt rename <old> <new>`) still works: the source
// name is never validated, only the destination.
func TestRenameFromInvalidLegacyNameSucceeds(t *testing.T) {
	s, kc := newStore(t)
	legacy := "weird name!"
	if err := kc.Set(KeychainService, legacy, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	st, _ := s.load()
	st.Accounts[legacy] = Meta{Email: "e@x.com"}
	if err := s.save(st); err != nil {
		t.Fatal(err)
	}
	if err := s.Rename(legacy, "fixed-name"); err != nil {
		t.Fatalf("Rename from a legacy invalid name should succeed, got %v", err)
	}
	if _, meta, err := s.Get("fixed-name"); err != nil || meta.Email != "e@x.com" {
		t.Errorf("renamed account wrong: %+v %v", meta, err)
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
