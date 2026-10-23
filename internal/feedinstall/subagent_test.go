package feedinstall

import (
	"strings"
	"testing"
)

var subagentEvents = append(append([]Event{}, events...),
	Event{Name: "SubagentStart", Timeout: 5},
	Event{Name: "SubagentStop", Timeout: 5},
	Event{Name: "PreToolUse", Matcher: "Agent|Task", Timeout: 5},
)

func TestAnOlderInstallPicksUpTheSubagentEventsAndUninstallRemovesThemExactly(t *testing.T) {
	r := newRig(t, userFile)
	r.install()
	r.opts.Events = subagentEvents

	p := r.install()
	if !p.Changed || p.Hooks != 3 {
		t.Fatalf("changed %v hooks %d", p.Changed, p.Hooks)
	}
	got := r.read()
	for _, ev := range []string{"SubagentStart", "SubagentStop", "PreToolUse"} {
		if c := commands(t, got, ev); len(c) != 1 || !strings.HasSuffix(c[0], Marker) {
			t.Errorf("%s: %v", ev, c)
		}
	}
	if !strings.Contains(got, `"matcher": "Agent|Task"`) && !strings.Contains(got, `"matcher":"Agent|Task"`) {
		t.Errorf("matcher missing: %s", got)
	}

	// a second install of the same set changes nothing
	again, err := Prepare(r.opts)
	if err != nil || again.Changed {
		t.Fatalf("changed %v err %v", again.Changed, err)
	}

	res, err := Uninstall(r.opts)
	if err != nil {
		t.Fatal(err)
	}
	// the file was rewritten by the second install, so this is the strip path
	if res.Restored {
		t.Log("restored from backup")
	}
	left := r.read()
	if strings.Contains(left, Marker) {
		t.Errorf("our entries left: %s", left)
	}
	for _, keep := range []string{"say done", "echo hi", "line.sh"} {
		if !strings.Contains(left, keep) {
			t.Errorf("lost %q: %s", keep, left)
		}
	}
	hooks, _ := parse(t, left)["hooks"].(map[string]any)
	for _, ev := range []string{"SubagentStart", "SubagentStop", "PreToolUse", "PostToolUse"} {
		if _, ok := hooks[ev]; ok {
			t.Errorf("%s left behind", ev)
		}
	}
}
