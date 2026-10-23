package account

import (
	"os"
	"strings"
	"testing"
)

func TestAnEmptyStateFileIsAnErrorNotNoAccounts(t *testing.T) {
	s, _ := newStore(t)
	if err := os.WriteFile(s.statePath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := s.List(); err == nil || !strings.Contains(err.Error(), s.statePath) {
		t.Errorf("List err = %v, want one that names %s", err, s.statePath)
	}
	// a write must not go on top of a file nobody could read
	if _, err := s.Create("robin", noFolders); err == nil {
		t.Error("Create wrote over an empty state file")
	}
	if err := s.SetBudget("robin", 1, 1); err == nil || strings.Contains(err.Error(), "not found") {
		t.Errorf("SetBudget err = %v, want the empty file reported, not a missing account", err)
	}
	if fi, err := os.Stat(s.statePath); err != nil || fi.Size() != 0 {
		t.Errorf("the empty file was changed: %v, %v", fi, err)
	}
}

func TestAMissingStateFileIsStillNoAccounts(t *testing.T) {
	s, _ := newStore(t)
	list, err := s.List()
	if err != nil || len(list) != 0 {
		t.Errorf("List = %v, %v", list, err)
	}
}
