package account

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

// set gives the account these rules, or fails the test.
func set(t *testing.T, s *Store, id string, r Rules) {
	t.Helper()
	if err := s.UpdateRules(id, func(Rules) (Rules, error) { return r, nil }); err != nil {
		t.Fatal(err)
	}
}

func rulesOf(t *testing.T, s *Store, id string) Rules {
	t.Helper()
	r, err := s.RulesOf(id)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAnAccountWithNoRulesMayServeAnyFolderAndDoesNotAutoSwitch(t *testing.T) {
	s := newMetaStore(t)
	acc := mustCreate(t, s, "work")

	got := rulesOf(t, s, acc.Name)

	if len(got.Folders) != 0 || got.AutoSwitch {
		t.Errorf("rules = %+v, want none", got)
	}
	if all, err := s.AllRules(); err != nil || len(all) != 0 {
		t.Errorf("AllRules = %v, %v, want none", all, err)
	}
}

func TestUpdateRulesSavesThemAndTheyReadBack(t *testing.T) {
	s := newMetaStore(t)
	acc := mustCreate(t, s, "work")

	err := s.UpdateRules(acc.Name, func(r Rules) (Rules, error) {
		r.Folders = append(r.Folders, "/srv/work", "/srv/clients")
		r.AutoSwitch = true
		return r, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	want := Rules{Folders: []string{"/srv/work", "/srv/clients"}, AutoSwitch: true}
	if got := rulesOf(t, s, acc.Name); !reflect.DeepEqual(got, want) {
		t.Errorf("rules = %+v, want %+v", got, want)
	}
	// a second store on the same file sees them: they were written, not kept
	again := New(s.statePath, s.logPath)
	all, err := again.AllRules()
	if err != nil || !reflect.DeepEqual(all, map[string]Rules{acc.Name: want}) {
		t.Errorf("a fresh store reads %v, %v", all, err)
	}
}

// the function gets a copy, so what it does to the slice cannot reach what is
// stored unless it returns it
func TestUpdateRulesHandsOverACopy(t *testing.T) {
	s := newMetaStore(t)
	acc := mustCreate(t, s, "work")
	set(t, s, acc.Name, Rules{Folders: []string{"/a", "/b"}})

	err := s.UpdateRules(acc.Name, func(r Rules) (Rules, error) {
		r.Folders[0] = "/changed"
		return r, errors.New("not this time")
	})

	if err == nil {
		t.Fatal("the error of the function was lost")
	}
	if got := rulesOf(t, s, acc.Name).Folders; !reflect.DeepEqual(got, []string{"/a", "/b"}) {
		t.Errorf("folders = %v after a refused update", got)
	}
}

func TestUpdateRulesWithAnErrorWritesNothing(t *testing.T) {
	s := newMetaStore(t)
	acc := mustCreate(t, s, "work")
	before, err := os.ReadFile(s.statePath)
	if err != nil {
		t.Fatal(err)
	}

	err = s.UpdateRules(acc.Name, func(r Rules) (Rules, error) {
		return Rules{Folders: []string{"/x"}, AutoSwitch: true}, errors.New("refused")
	})

	if err == nil || err.Error() != "refused" {
		t.Errorf("error = %v, want the one the function gave", err)
	}
	after, _ := os.ReadFile(s.statePath)
	if string(after) != string(before) {
		t.Errorf("the file changed:\n%s\nthen\n%s", before, after)
	}
}

func TestRulesOfAnUnknownAccountAreAnError(t *testing.T) {
	s := newMetaStore(t)
	var missing *NotFoundError

	if _, err := s.RulesOf("nobody"); !errors.As(err, &missing) {
		t.Errorf("RulesOf: error = %v, want not found", err)
	}
	err := s.UpdateRules("nobody", func(r Rules) (Rules, error) { return r, nil })
	if !errors.As(err, &missing) {
		t.Errorf("UpdateRules: error = %v, want not found", err)
	}
	if _, statErr := os.Stat(s.statePath); statErr == nil {
		t.Error("a refused update made the file")
	}
}

// the key is only in the file once an account has rules, so a file nobody set
// rules in looks as it did before, and one whose rules were cleared goes back
func TestRulesAreLeftOutOfTheFileWhenThereAreNone(t *testing.T) {
	s := newMetaStore(t)
	acc := mustCreate(t, s, "work")

	raw, _ := os.ReadFile(s.statePath)
	if strings.Contains(string(raw), "rules") {
		t.Errorf("a new account has the rules key:\n%s", raw)
	}

	set(t, s, acc.Name, Rules{Folders: []string{"/a"}, AutoSwitch: true})
	raw, _ = os.ReadFile(s.statePath)
	for _, want := range []string{`"rules"`, `"folders"`, `"autoSwitch": true`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the file lacks %s:\n%s", want, raw)
		}
	}

	set(t, s, acc.Name, Rules{})
	raw, _ = os.ReadFile(s.statePath)
	if strings.Contains(string(raw), "rules") {
		t.Errorf("cleared rules are still in the file:\n%s", raw)
	}
}

func TestStateWithRulesIsStillReadableByV010(t *testing.T) {
	s := newMetaStore(t)
	acc := mustCreate(t, s, "work")
	if err := s.SetIdentity(acc.Name, Identity{Email: "w@example.com", Plan: "max"}); err != nil {
		t.Fatal(err)
	}
	set(t, s, acc.Name, Rules{Folders: []string{"/srv/work"}, AutoSwitch: true})
	raw, err := os.ReadFile(s.statePath)
	if err != nil {
		t.Fatal(err)
	}

	var old v010State
	if err := json.Unmarshal(raw, &old); err != nil {
		t.Fatalf("v0.1.0 cannot read the file: %v", err)
	}
	got := old.Accounts["work"]
	if got.Email != "w@example.com" || got.Plan != "max" || got.AddedAt.IsZero() {
		t.Errorf("v0.1.0 sees %+v", got)
	}
}

func TestRulesOfOneAccountAreNotThoseOfAnother(t *testing.T) {
	s := newMetaStore(t)
	work, home := mustCreate(t, s, "work"), mustCreate(t, s, "home")
	set(t, s, work.Name, Rules{Folders: []string{"/srv/work"}, AutoSwitch: true})
	set(t, s, home.Name, Rules{Folders: []string{"/srv/home"}})

	set(t, s, work.Name, Rules{})

	if got := rulesOf(t, s, home.Name); !reflect.DeepEqual(got, Rules{Folders: []string{"/srv/home"}}) {
		t.Errorf("home = %+v", got)
	}
}

// every other change loads the whole file and writes it back, so what it does
// not know about has to survive it
func TestNothingElseThatTheStoreDoesDropsTheRules(t *testing.T) {
	s := newMetaStore(t)
	work, other := mustCreate(t, s, "work"), mustCreate(t, s, "other")
	want := Rules{Folders: []string{"/srv/work"}, AutoSwitch: true}
	set(t, s, work.Name, want)
	set(t, s, other.Name, want)

	steps := []struct {
		name string
		do   func() error
	}{
		{"Touch", func() error { return s.Touch(work.Name) }},
		{"SetIdentity", func() error { return s.SetIdentity(work.Name, Identity{Email: "w@example.com"}) }},
		{"ClearSignIn", func() error { return s.ClearSignIn(work.Name) }},
		{"Relabel", func() error { return s.Relabel(work.Name, "office") }},
		{"SetBudget", func() error { return s.SetBudget(work.Name, 5, 10) }},
		{"Hide", func() error { return s.Hide(work.Name) }},
		{"Unhide", func() error { return s.Unhide(work.Name) }},
		{"Create", func() error { _, err := s.Create("fresh", noFolders); return err }},
	}
	for _, step := range steps {
		if err := step.do(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		for _, id := range []string{work.Name, other.Name} {
			if got := rulesOf(t, s, id); !reflect.DeepEqual(got, want) {
				t.Errorf("after %s the rules of %s are %+v", step.name, id, got)
			}
		}
	}
}
