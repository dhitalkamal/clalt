package cli

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/account"
)

func TestRulesJSON(t *testing.T) {
	e := newEnv(t)
	addAccount(t, e, "work")
	dir := subfolder(t, "work-projects")
	gone := filepath.Join(t.TempDir(), "gone")
	e.setRules("work", account.Rules{Folders: []string{dir, gone}, AutoSwitch: true})

	if code := e.run("rules", "work", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	var got map[string]any
	if err := json.Unmarshal(e.out.Bytes(), &got); err != nil {
		t.Fatalf("not json: %v\n%s", err, e.out.String())
	}
	want := map[string]any{
		"id": "work", "label": "work", "folders": []any{dir, gone}, "anyFolder": false, "autoSwitch": true,
		"problems": []any{map[string]any{"folder": gone, "why": "does not exist"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("json = %v\nwant   %v", got, want)
	}
}

// an empty list is [] and not null, so that a script can loop over it
func TestRulesJSONOfAnAccountWithNoRulesHasEmptyLists(t *testing.T) {
	e := newEnv(t)
	addAccount(t, e, "work")

	if code := e.run("rules", "work", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	var got struct {
		Folders   []string `json:"folders"`
		AnyFolder bool     `json:"anyFolder"`
		Problems  []any    `json:"problems"`
	}
	if err := json.Unmarshal(e.out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Folders == nil || got.Problems == nil || !got.AnyFolder {
		t.Errorf("json = %+v from %s", got, e.out.String())
	}
}

func TestRulesJSONAfterAChangeIsOnlyTheJSON(t *testing.T) {
	e := newEnv(t)
	addAccount(t, e, "work")
	dir := subfolder(t, "x")

	if code := e.run("rules", "work", "--add-folder", dir, "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	var got map[string]any
	if err := json.Unmarshal(e.out.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not only json: %v\n%s", err, e.out.String())
	}
}

func TestRulesNeverAsksClaude(t *testing.T) {
	e := newEnv(t)
	addAccount(t, e, "work")
	dir := subfolder(t, "x")

	e.run("rules", "work", "--add-folder", dir, "--auto-switch")
	e.run("rules", "work")

	if calls := e.claude.calls("status"); len(calls) != 0 {
		t.Errorf("claude was asked: %v", calls)
	}
}

// what the state file holds can be anything, it is printed to a terminal
func TestRulesCleansWhatItPrints(t *testing.T) {
	e := newEnv(t)
	addAccount(t, e, "work")
	e.setRules("work", account.Rules{Folders: []string{"/tmp/a\x1b[2Jb"}})

	if code := e.run("rules", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if hasControl(e.out.String()) {
		t.Errorf("the listing has a control character: %q", e.out.String())
	}
}

func TestHelpListsTheRulesCommand(t *testing.T) {
	e := newEnv(t)

	e.run("help")

	for _, want := range []string{"clalt rules <name>", "--add-folder", "--auto-switch"} {
		if !strings.Contains(e.out.String(), want) {
			t.Errorf("the help is missing %q:\n%s", want, e.out.String())
		}
	}
}

func TestRulesOfAnUnknownOrRemovedAccount(t *testing.T) {
	e := newEnv(t)
	e.signIn("gone", "g@example.com")
	if code := e.run("remove", "gone", "--force"); code != 0 {
		t.Fatalf("remove exited %d: %s", code, e.errOut.String())
	}
	e.reset()

	if code := e.run("rules", "nobody"); code != 1 || !strings.Contains(e.errOut.String(), `no account called "nobody"`) {
		t.Errorf("unknown: exit %d: %s", code, e.errOut.String())
	}
	e.reset()
	if code := e.run("rules", "gone", "--auto-switch"); code != 1 || !strings.Contains(e.errOut.String(), "was removed") {
		t.Errorf("removed: exit %d: %s", code, e.errOut.String())
	}
}

func TestRulesLabelsAreMatchedIgnoringCase(t *testing.T) {
	e := newEnv(t)
	addAccount(t, e, "Work")

	if code := e.run("rules", "WORK", "--auto-switch"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if !e.rulesOf("work").AutoSwitch {
		t.Error("the rules of Work were not changed")
	}
}
