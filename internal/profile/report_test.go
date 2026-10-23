package profile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportErrJoinsTheItemErrors(t *testing.T) {
	boom, bang := errors.New("boom"), errors.New("bang")
	rep := Report{Errors: []ItemError{{Name: "a", Err: boom}, {Name: "b", Err: bang}}}

	err := rep.Err()

	if !errors.Is(err, boom) || !errors.Is(err, bang) {
		t.Errorf("Err() = %v, want both causes reachable", err)
	}
	for _, want := range []string{"a: boom", "b: bang"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Err() = %q, missing %q", err, want)
		}
	}
	if err := (Report{Linked: []string{"x"}}).Err(); err != nil {
		t.Errorf("a report without errors has Err() = %v", err)
	}
}

func TestReportComplete(t *testing.T) {
	cases := []struct {
		name string
		rep  Report
		want bool
	}{
		{"nothing to do", Report{}, true},
		{"changes only", Report{Linked: []string{"a"}, Repointed: []Step{{Name: "b"}}, Removed: []string{"c"}}, true},
		{"something kept", Report{Kept: []Step{{Name: "a"}}}, false},
		{"an error", Report{Errors: []ItemError{{Name: "a", Err: errors.New("x")}}}, false},
	}
	for _, c := range cases {
		if got := c.rep.Complete(); got != c.want {
			t.Errorf("%s: Complete() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestPlanSetsTheOldTargetOfLinksItReplacesOrRemoves(t *testing.T) {
	root, shared := sharedFixture(t)
	acct := filepath.Join(root, "acct")
	linkAll(t, acct, shared)
	other := filepath.Join(root, "other")
	mkdirT(t, other)
	if err := os.Remove(filepath.Join(acct, "commands")); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, other, filepath.Join(acct, "commands"))
	mustSymlink(t, filepath.Join(shared, "sessions"), filepath.Join(acct, "sessions"))

	got := stepMap(mustPlan(t, acct, shared))

	if got["commands"].Old != other {
		t.Errorf("repoint Old = %q, want %q", got["commands"].Old, other)
	}
	if got["sessions"].Old != filepath.Join(shared, "sessions") {
		t.Errorf("remove Old = %q", got["sessions"].Old)
	}
}
