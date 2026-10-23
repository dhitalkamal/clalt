package account

import (
	"path/filepath"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return New(filepath.Join(dir, "state.json"), filepath.Join(dir, "switches.jsonl"))
}

func TestListIsSortedById(t *testing.T) {
	s := newStore(t)
	for _, label := range []string{"b", "a", "c"} {
		mustCreate(t, s, label)
	}

	list, err := s.List()

	if err != nil || len(list) != 3 {
		t.Fatalf("List = %v, %v", list, err)
	}
	for i, id := range []string{"a", "b", "c"} {
		if list[i].Name != id {
			t.Errorf("list[%d] = %+v, want %s", i, list[i], id)
		}
	}
}

func TestSetBudgetSetsClearsAndLeavesCaps(t *testing.T) {
	s := newStore(t)
	mustCreate(t, s, "work")

	if err := s.SetBudget("work", 20, 100); err != nil {
		t.Fatal(err)
	}
	if err := s.SetBudget("work", -1, 50); err != nil { // a negative value leaves that cap
		t.Fatal(err)
	}
	m := stored(t, s, "work").Meta
	if m.DailyBudget != 20 || m.WeeklyBudget != 50 {
		t.Errorf("caps = %v / %v, want 20 / 50", m.DailyBudget, m.WeeklyBudget)
	}
	if err := s.SetBudget("work", 0, 0); err != nil { // zero clears
		t.Fatal(err)
	}
	m = stored(t, s, "work").Meta
	if m.DailyBudget != 0 || m.WeeklyBudget != 0 {
		t.Errorf("caps = %v / %v, want none", m.DailyBudget, m.WeeklyBudget)
	}
	if err := s.SetBudget("nobody", 1, 1); err == nil {
		t.Error("SetBudget accepted an unknown account")
	}
}
