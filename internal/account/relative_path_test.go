package account

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// with no home the path of the state file is relative, which is the working
// directory, and clalt must not read or write there: it could pick up somebody
// else's file or leave its own behind. the store refuses before it touches
// anything, the lock file included.
func TestARelativeStatePathIsRefusedBeforeAnythingIsTouched(t *testing.T) {
	t.Chdir(t.TempDir())
	s := New(filepath.Join(".config", "clalt", "state.json"), filepath.Join(".config", "clalt", "switches.jsonl"))

	calls := map[string]func() error{
		"List":         func() error { _, err := s.List(); return err },
		"Create":       func() error { _, err := s.Create("robin", noFolders); return err },
		"SetBudget":    func() error { return s.SetBudget("robin", 1, 1) },
		"SetIdentity":  func() error { return s.SetIdentity("robin", Identity{Email: "k@example.com"}) },
		"Hide":         func() error { return s.Hide("robin") },
		"Touch":        func() error { return s.Touch("robin") },
		"ResolveLabel": func() error { _, _, err := s.ResolveLabel("robin"); return err },
		"ThinSince":    func() error { _, err := s.ThinSince(); return err },
		"ReadSwitchLog": func() error {
			_, err := s.ReadSwitchLog()
			return err
		},
	}
	for name, call := range calls {
		if err := call(); err == nil || !strings.Contains(err.Error(), "HOME") {
			t.Errorf("%s: err = %v, want a refusal that says HOME is not set", name, err)
		}
	}
	if entries, _ := os.ReadDir("."); len(entries) != 0 {
		t.Errorf("the store left %d entries in the working directory: %v", len(entries), entries)
	}
}

func TestAnAbsoluteStatePathStillWorks(t *testing.T) {
	s := newMetaStore(t)

	if _, err := s.List(); err != nil {
		t.Errorf("List: %v", err)
	}
}
