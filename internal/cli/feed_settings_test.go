package cli

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

type parsedSettings struct {
	Hooks map[string][]struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Type    string `json:"type"`
			Command string `json:"command"`
			Timeout int    `json:"timeout"`
			Async   bool   `json:"async"`
		} `json:"hooks"`
	} `json:"hooks"`
	StatusLine map[string]any `json:"statusLine"`
}

func parseSettings(t *testing.T, raw string) parsedSettings {
	t.Helper()
	var p parsedSettings
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("not json: %v\n%s", err, raw)
	}
	return p
}

func TestShQuote(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix quoting")
	}
	for in, want := range map[string]string{
		"/usr/bin/clalt":        `'/usr/bin/clalt'`,
		"/my dir/clalt":         `'/my dir/clalt'`,
		"/it's/clalt":           `'/it'\''s/clalt'`,
		"/a$b/`c`/clalt":        "'/a$b/`c`/clalt'",
		"/tmp/x;rm -rf ~/clalt": `'/tmp/x;rm -rf ~/clalt'`,
	} {
		if got := shQuote(in); got != want {
			t.Errorf("shQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestFeedSettingsInstallsEveryEventWithTheQuotedBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix quoting")
	}
	raw, err := feedSettings("/my dir/it's & clalt", nil)
	if err != nil {
		t.Fatal(err)
	}
	p := parseSettings(t, raw)

	want := map[string]struct {
		matcher string
		timeout int
		async   bool
	}{
		"SessionStart": {"", 5, false}, "UserPromptSubmit": {"", 5, false}, "Stop": {"", 5, false},
		"StopFailure":  {"", 5, false},
		"Notification": {"permission_prompt|elicitation_dialog|elicitation_url_dialog|agent_needs_input", 5, false},
		"SessionEnd":   {"", 1, false}, "PostToolUse": {"", 0, true}, "PostToolUseFailure": {"", 0, true},
	}
	if len(p.Hooks) != len(want) {
		t.Errorf("%d events, want %d", len(p.Hooks), len(want))
	}
	for event, w := range want {
		groups := p.Hooks[event]
		if len(groups) != 1 || len(groups[0].Hooks) != 1 {
			t.Fatalf("%s: %+v", event, groups)
		}
		g, h := groups[0], groups[0].Hooks[0]
		if g.Matcher != w.matcher || h.Timeout != w.timeout || h.Async != w.async || h.Type != "command" {
			t.Errorf("%s: %+v", event, g)
		}
		if h.Command != `'/my dir/it'\''s & clalt' hook `+event {
			t.Errorf("%s command %q", event, h.Command)
		}
	}
	if p.StatusLine["type"] != "command" || p.StatusLine["command"] != `'/my dir/it'\''s & clalt' statusline` {
		t.Errorf("statusLine %v", p.StatusLine)
	}
}

func TestFeedSettingsCopiesHowTheUsersStatusLineIsDrawn(t *testing.T) {
	user := &userStatusLine{Command: "x", Extras: map[string]json.RawMessage{
		"padding": json.RawMessage(`2`), "refreshInterval": json.RawMessage(`5`), "hideVimModeIndicator": json.RawMessage(`true`),
	}}
	raw, err := feedSettings("/bin/clalt", user)
	if err != nil {
		t.Fatal(err)
	}
	sl := parseSettings(t, raw).StatusLine
	if sl["padding"] != float64(2) || sl["refreshInterval"] != float64(5) || sl["hideVimModeIndicator"] != true || sl["command"] == "x" {
		t.Errorf("statusLine %v", sl)
	}
}

func TestHasSettingsFlag(t *testing.T) {
	for args, want := range map[string]bool{
		"":                     false,
		"--model opus":         false,
		"--settings a.json":    true,
		"--settings={}":        true,
		"-p --settings {}":     true,
		"--settings-sources x": false,
	} {
		if got := hasSettingsFlag(strings.Fields(args)); got != want {
			t.Errorf("hasSettingsFlag(%q) = %v, want %v", args, got, want)
		}
	}
}
