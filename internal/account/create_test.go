package account

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func newMetaStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s := New(filepath.Join(dir, "state.json"), filepath.Join(dir, "switches.jsonl"))
	s.now = func() time.Time { return fixedNow }
	return s
}

func mustCreate(t *testing.T, s *Store, label string) Account {
	t.Helper()
	acc, err := s.Create(label, noFolders)
	if err != nil {
		t.Fatalf("Create(%q): %v", label, err)
	}
	return acc
}

// noFolders says no folder exists for any id.
func noFolders(string) (bool, error) { return false, nil }

// addLegacy plants an account the way v0.1.0 stored it: no label.
func addLegacy(t *testing.T, s *Store, id string, m Meta) {
	t.Helper()
	st, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	st.Accounts[id] = m
	if err := s.save(st); err != nil {
		t.Fatal(err)
	}
}

func stored(t *testing.T, s *Store, id string) Account {
	t.Helper()
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range list {
		if a.Name == id {
			return a
		}
	}
	t.Fatalf("no account with id %q in %v", id, list)
	return Account{}
}

func TestCreateUsesTheLabelAsTheId(t *testing.T) {
	s := newMetaStore(t)

	acc, err := s.Create("robin", noFolders)

	if err != nil {
		t.Fatal(err)
	}
	if acc.Name != "robin" || acc.Label() != "robin" || acc.Meta.Label != "robin" || !acc.Meta.AddedAt.Equal(fixedNow) {
		t.Errorf("account = %+v", acc)
	}
	if got := stored(t, s, "robin"); !reflect.DeepEqual(got, acc) {
		t.Errorf("stored %+v, returned %+v", got, acc)
	}
}

func TestCreateRejectsInvalidLabels(t *testing.T) {
	s := newMetaStore(t)
	for _, label := range []string{"", "../x", ".hidden", "-flag", "con", "has space", "a/b", strings.Repeat("a", maxNameLen+1)} {
		if acc, err := s.Create(label, noFolders); err == nil {
			t.Errorf("Create(%q) = %+v, want an error", label, acc)
		}
	}
	if _, err := os.Stat(s.statePath); !os.IsNotExist(err) {
		t.Errorf("state.json was written for rejected labels (stat err: %v)", err)
	}
}

func TestMaxNameLenMatchesValidateName(t *testing.T) {
	if err := ValidateName(strings.Repeat("a", maxNameLen)); err != nil {
		t.Errorf("a name of %d chars is rejected: %v", maxNameLen, err)
	}
	if err := ValidateName(strings.Repeat("a", maxNameLen+1)); err == nil {
		t.Errorf("a name of %d chars is accepted", maxNameLen+1)
	}
}

func TestCreateRejectsALabelInUseIgnoringCase(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")
	for _, label := range []string{"robin", "Robin", "ROBIN"} {
		if _, err := s.Create(label, noFolders); err == nil {
			t.Errorf("Create(%q) succeeded with robin in use", label)
		}
	}

	// a removed account keeps its label
	if err := s.Hide("robin"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("Robin", noFolders); err == nil {
		t.Error("Create reused the label of a hidden account")
	}

	// an account from v0.1.0 has no label, its id is the label
	addLegacy(t, s, "work", Meta{})
	if _, err := s.Create("WORK", noFolders); err == nil {
		t.Error("Create reused the id of a legacy account as a label")
	}
	if list, _ := s.List(); len(list) != 2 {
		t.Errorf("accounts = %d, want 2", len(list))
	}
}

func TestCreateNumbersTheIdWhenItIsTaken(t *testing.T) {
	s := newMetaStore(t)
	first := mustCreate(t, s, "robin")
	// the account moves on to a new label but keeps its folder, so the id stays taken
	if err := s.Relabel(first.Name, "work"); err != nil {
		t.Fatal(err)
	}

	second := mustCreate(t, s, "robin")
	if second.Name != "robin-2" || second.Label() != "robin" {
		t.Errorf("second = id %q label %q, want robin-2 and robin", second.Name, second.Label())
	}
	if err := s.Relabel(second.Name, "home"); err != nil {
		t.Fatal(err)
	}
	if third := mustCreate(t, s, "robin"); third.Name != "robin-3" {
		t.Errorf("third id = %q, want robin-3", third.Name)
	}
}

func TestCreateComparesIdsIgnoringCase(t *testing.T) {
	s := newMetaStore(t)
	first := mustCreate(t, s, "Robin")
	if err := s.Relabel(first.Name, "work"); err != nil {
		t.Fatal(err)
	}
	// macOS would put both in one folder
	if got := mustCreate(t, s, "robin"); got.Name != "robin-2" {
		t.Errorf("id = %q, want robin-2", got.Name)
	}
}

func TestCreateSkipsIdsWhoseFolderExists(t *testing.T) {
	s := newMetaStore(t)
	var asked []string
	taken := map[string]bool{"robin": true, "robin-2": true}

	acc, err := s.Create("robin", func(id string) (bool, error) {
		asked = append(asked, id)
		return taken[id], nil
	})

	if err != nil {
		t.Fatal(err)
	}
	if acc.Name != "robin-3" || acc.Label() != "robin" {
		t.Errorf("account = id %q label %q, want robin-3 and robin", acc.Name, acc.Label())
	}
	if !reflect.DeepEqual(asked, []string{"robin", "robin-2", "robin-3"}) {
		t.Errorf("asked about %v", asked)
	}
}

// a check written as an os.Lstat returns the error too, and "does not exist" is
// the good answer.
func TestCreateTreatsNotExistAsAFreeId(t *testing.T) {
	s := newMetaStore(t)
	lstat := func(id string) (bool, error) {
		if id == "robin" {
			return true, nil
		}
		return false, &fs.PathError{Op: "lstat", Path: id, Err: fs.ErrNotExist}
	}

	acc, err := s.Create("robin", lstat)

	if err != nil || acc.Name != "robin-2" {
		t.Errorf("Create = %+v, %v, want id robin-2", acc, err)
	}
}

func TestCreateNeedsAFolderCheckAndStopsWhenItFails(t *testing.T) {
	s := newMetaStore(t)
	if acc, err := s.Create("robin", nil); err == nil {
		t.Errorf("Create without a folder check = %+v", acc)
	}
	boom := errors.New("permission denied")
	acc, err := s.Create("robin", func(string) (bool, error) { return false, boom })
	if !errors.Is(err, boom) {
		t.Errorf("Create = %+v, %v, want the check's error", acc, err)
	}
	if _, err := os.Stat(s.statePath); !os.IsNotExist(err) {
		t.Error("state.json was written although the folders could not be checked")
	}
}

func TestCreateKeepsTheNumberedIdWithinTheNameLimit(t *testing.T) {
	s := newMetaStore(t)
	long := strings.Repeat("a", maxNameLen)
	first := mustCreate(t, s, long)
	if err := s.Relabel(first.Name, "short"); err != nil {
		t.Fatal(err)
	}

	acc := mustCreate(t, s, long)

	if err := ValidateName(acc.Name); err != nil || acc.Name == first.Name || !strings.HasSuffix(acc.Name, "-2") {
		t.Errorf("id = %q (%v), want a valid -2 id that differs from %q", acc.Name, err, first.Name)
	}
}

func TestCreateGivesUpWhenNoIdIsFree(t *testing.T) {
	s := newMetaStore(t)
	if acc, err := s.Create("robin", func(string) (bool, error) { return true, nil }); err == nil {
		t.Errorf("Create = %+v, want an error", acc)
	}
	if _, err := os.Stat(s.statePath); !os.IsNotExist(err) {
		t.Error("state.json was written without an account")
	}
}

func TestCreateLeavesTheRestOfTheStateAlone(t *testing.T) {
	s := newMetaStore(t)
	addLegacy(t, s, "work", Meta{Email: "w@example.com", DailyBudget: 25})
	st, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	st.Active = "work"
	if err := s.save(st); err != nil {
		t.Fatal(err)
	}

	mustCreate(t, s, "fresh")

	if st, _ := s.load(); st.Active != "work" {
		t.Errorf("active pointer = %q, want work", st.Active)
	}
	if got := stored(t, s, "work"); got.Meta.Email != "w@example.com" || got.Meta.DailyBudget != 25 {
		t.Errorf("work changed: %+v", got)
	}
}

// a removed account keeps its folder, so its id stays taken even after it is
// relabeled, and a new account under the label it had gets the next number
func TestCreateNumbersTheIdWhenAHiddenAccountHoldsIt(t *testing.T) {
	s := newMetaStore(t)
	first := mustCreate(t, s, "robin")
	if err := s.Hide(first.Name); err != nil {
		t.Fatal(err)
	}
	if err := s.Relabel(first.Name, "old"); err != nil {
		t.Fatal(err)
	}

	second := mustCreate(t, s, "robin")

	if second.Name != "robin-2" {
		t.Errorf("id = %q, want robin-2", second.Name)
	}
	if got := stored(t, s, "robin"); got.Label() != "old" || !got.Meta.Hidden {
		t.Errorf("the removed account was overwritten: %+v", got)
	}
}
