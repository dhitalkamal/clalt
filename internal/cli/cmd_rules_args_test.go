package cli

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseRulesArgs(t *testing.T) {
	yes, no := true, false
	cases := []struct {
		name string
		args []string
		want rulesRequest
		err  string // empty means it parses
	}{
		{"just the account", []string{"work"}, rulesRequest{name: "work"}, ""},
		{"add one folder", []string{"work", "--add-folder", "/a"}, rulesRequest{name: "work", add: []string{"/a"}}, ""},
		{"flags before the name", []string{"--add-folder", "/a", "work"}, rulesRequest{name: "work", add: []string{"/a"}}, ""},
		{"several adds and removes", []string{"work", "--add-folder", "/a", "--add-folder", "/b", "--remove-folder", "/c"},
			rulesRequest{name: "work", add: []string{"/a", "/b"}, remove: []string{"/c"}}, ""},
		{"auto-switch on", []string{"work", "--auto-switch"}, rulesRequest{name: "work", autoSwitch: &yes}, ""},
		{"auto-switch off", []string{"work", "--no-auto-switch"}, rulesRequest{name: "work", autoSwitch: &no}, ""},
		{"the same switch twice is fine", []string{"work", "--auto-switch", "--auto-switch"}, rulesRequest{name: "work", autoSwitch: &yes}, ""},
		{"on and off together", []string{"work", "--auto-switch", "--no-auto-switch"}, rulesRequest{}, "both"},
		{"no account", nil, rulesRequest{}, "which account"},
		{"flags and no account", []string{"--auto-switch"}, rulesRequest{}, "which account"},
		{"a second account", []string{"work", "home"}, rulesRequest{}, "unexpected argument home"},
		{"an unknown flag", []string{"work", "--nope"}, rulesRequest{}, "unknown flag --nope"},
		{"a flag that is not quite one", []string{"work", "--add"}, rulesRequest{}, "unknown flag --add"},
		{"add with nothing after it", []string{"work", "--add-folder"}, rulesRequest{}, "--add-folder needs a folder"},
		{"remove with nothing after it", []string{"work", "--remove-folder"}, rulesRequest{}, "--remove-folder needs a folder"},
	}
	for _, c := range cases {
		got, err := parseRulesArgs(c.args)
		if c.err != "" {
			var bad usageError
			if !errors.As(err, &bad) || !strings.Contains(bad.msg, c.err) || bad.usage != rulesUsage {
				t.Errorf("%s: error = %v, want a usage error with %q", c.name, err, c.err)
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: parseRulesArgs(%v) = %+v, %v, want %+v", c.name, c.args, got, err, c.want)
		}
	}
}

func TestARulesRequestKnowsWhetherItChangesAnything(t *testing.T) {
	yes := true
	for req, want := range map[string]bool{
		"none":   false,
		"add":    true,
		"remove": true,
		"switch": true,
	} {
		r := rulesRequest{name: "work"}
		switch req {
		case "add":
			r.add = []string{"/a"}
		case "remove":
			r.remove = []string{"/a"}
		case "switch":
			r.autoSwitch = &yes
		}
		if got := r.changes(); got != want {
			t.Errorf("%s: changes() = %v, want %v", req, got, want)
		}
	}
}
