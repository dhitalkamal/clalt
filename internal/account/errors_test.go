package account

import (
	"errors"
	"strings"
	"testing"
)

func TestALabelThatIsTakenSaysWhoHasItAndWhetherTheyAreHidden(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")
	mustCreate(t, s, "work")

	_, err := s.Create("ROBIN", noFolders)

	var taken *LabelTakenError
	if !errors.As(err, &taken) || taken.ID != "robin" || taken.Label != "ROBIN" || taken.Hidden {
		t.Fatalf("err = %v, want a LabelTakenError for robin, visible", err)
	}
	if strings.Contains(err.Error(), "removed") {
		t.Errorf("a visible account is not described as removed: %q", err)
	}

	if err := s.Hide("robin"); err != nil {
		t.Fatal(err)
	}
	for name, err := range map[string]error{
		"Create":  func() error { _, err := s.Create("robin", noFolders); return err }(),
		"Relabel": s.Relabel("work", "Robin"),
	} {
		if !errors.As(err, &taken) || taken.ID != "robin" || !taken.Hidden {
			t.Errorf("%s: err = %v, want a LabelTakenError for the hidden robin", name, err)
		}
		if err != nil && !strings.Contains(err.Error(), "removed") {
			t.Errorf("%s: %q should say the holder was removed", name, err)
		}
	}
}

func TestAnAccountThatDoesNotExistIsATypedError(t *testing.T) {
	s := newMetaStore(t)
	mustCreate(t, s, "robin")
	calls := map[string]error{
		"SetIdentity": s.SetIdentity("ghost", Identity{Email: "g@example.com"}),
		"ClearSignIn": s.ClearSignIn("ghost"),
		"Hide":        s.Hide("ghost"),
		"Unhide":      s.Unhide("ghost"),
		"Relabel":     s.Relabel("ghost", "fresh"),
		"Touch":       s.Touch("ghost"),
		"SetBudget":   s.SetBudget("ghost", 1, 1),
	}
	for name, err := range calls {
		var nf *NotFoundError
		if !errors.As(err, &nf) || nf.ID != "ghost" {
			t.Errorf("%s: err = %v, want a NotFoundError for ghost", name, err)
		}
	}
	// the wording is the one the older methods always had
	if got := (&NotFoundError{ID: "ghost"}).Error(); got != `account "ghost" not found` {
		t.Errorf("message = %q", got)
	}
}
