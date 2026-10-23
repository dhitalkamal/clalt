package account

import (
	"reflect"
	"testing"
)

func TestCaseCollisionsListsIdsThatShareAFolderWhereCaseIsFolded(t *testing.T) {
	s := newMetaStore(t)
	// v0.1.0 never checked, so a state file can hold all of these
	for _, id := range []string{"Robin", "robin", "ROBIN", "work", "Home", "home", "alone"} {
		addLegacy(t, s, id, Meta{})
	}

	got, err := s.CaseCollisions()

	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"Home", "home"}, {"ROBIN", "Robin", "robin"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CaseCollisions = %v, want %v", got, want)
	}
}

func TestCaseCollisionsIsEmptyForAStoreWithoutAny(t *testing.T) {
	s := newMetaStore(t)
	if got, err := s.CaseCollisions(); err != nil || len(got) != 0 {
		t.Errorf("no state file: %v, %v", got, err)
	}
	mustCreate(t, s, "robin")
	mustCreate(t, s, "work")
	if got, err := s.CaseCollisions(); err != nil || len(got) != 0 {
		t.Errorf("clean store: %v, %v", got, err)
	}
}
