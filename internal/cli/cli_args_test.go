package cli

import (
	"reflect"
	"testing"
)

func TestSplitJSONFlag(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		rest     []string
		jsonFlag bool
	}{
		{"none", []string{"list"}, []string{"list"}, false},
		{"anywhere before the double dash", []string{"list", "--json"}, []string{"list"}, true},
		{"leading", []string{"--json", "list"}, []string{"list"}, true},
		{"after the double dash it is claude's", []string{"run", "work", "--", "auth", "status", "--json"}, []string{"run", "work", "--", "auth", "status", "--json"}, false},
		{"before and after", []string{"run", "--json", "work", "--", "--json"}, []string{"run", "work", "--", "--json"}, true},
		{"nothing", nil, nil, false},
		// once the account of run is named, everything after it is claude's: the
		// README promises that, and claude has a --json of its own
		{"after the name of run", []string{"run", "work", "--json"}, []string{"run", "work", "--json"}, false},
		{"among the arguments for claude", []string{"run", "work", "-p", "hi", "--json", "--model", "opus"}, []string{"run", "work", "-p", "hi", "--json", "--model", "opus"}, false},
		{"leading for clalt, then claude's", []string{"--json", "run", "work", "--json"}, []string{"run", "work", "--json"}, true},
		{"before the name it is still clalt's", []string{"run", "--json", "work", "--model", "opus"}, []string{"run", "work", "--model", "opus"}, true},
		{"run without a name yet", []string{"run", "--json"}, []string{"run"}, true},
		{"a flag before the command", []string{"-x", "run", "work", "--json"}, []string{"-x", "run", "work", "--json"}, false},
		{"flags and --json before the name", []string{"run", "-x", "--json", "work"}, []string{"run", "-x", "work"}, true},
		{"other commands keep it anywhere", []string{"usage", "work", "--json"}, []string{"usage", "work"}, true},
		{"a name that is not run's account", []string{"list", "work", "--json"}, []string{"list", "work"}, true},
	}
	for _, c := range cases {
		rest, got := splitJSONFlag(c.args)
		if !reflect.DeepEqual(rest, c.rest) || got != c.jsonFlag {
			t.Errorf("%s: splitJSONFlag(%v) = %v, %v, want %v, %v", c.name, c.args, rest, got, c.rest, c.jsonFlag)
		}
	}
}

// what the end user sees: the session is started with the --json that was typed
// after the name, and clalt does not take it for its own
func TestRunPassesAJsonFlagAfterTheNameToClaude(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	if code := e.run("run", "work", "-p", "hi", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	argv, _, _ := e.claude.session()
	if want := []string{"-p", "hi", "--json"}; !reflect.DeepEqual(argv, want) {
		t.Errorf("claude was started with %v, want %v", argv, want)
	}
	if e.app.JSON {
		t.Error("clalt took the flag for itself")
	}
}

func TestRunStillTakesAJsonFlagBeforeTheName(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	if code := e.run("run", "--json", "work", "-p", "hi"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	argv, _, _ := e.claude.session()
	if want := []string{"-p", "hi"}; !reflect.DeepEqual(argv, want) {
		t.Errorf("claude was started with %v, want %v", argv, want)
	}
}
