package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
)

type settingsHook struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
	Async   bool   `json:"async,omitempty"`
}

type settingsGroup struct {
	Matcher string         `json:"matcher,omitempty"`
	Hooks   []settingsHook `json:"hooks"`
}

// shQuote quotes a path for the shell that claude runs hook and statusline
// commands with. on windows the path is double quoted with forward slashes,
// which Git Bash takes; that part is best effort.
func shQuote(s string) string {
	if runtime.GOOS == "windows" {
		return `"` + filepath.ToSlash(s) + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// feedSettings is the JSON that clalt run passes to claude with --settings: the
// hooks and the statusLine that feed the session to clalt, with bin as the
// clalt that runs them. hooks merge with the user's own, so theirs keep running.
// the statusLine is one object and replaces theirs, which is why clalt
// statusline runs their command itself; the keys that change how it is drawn
// are copied, so that it looks the same.
func feedSettings(bin string, user *userStatusLine) (string, error) {
	q := shQuote(bin)
	hooks := map[string][]settingsGroup{}
	for _, ev := range hookEvents {
		hooks[ev.name] = []settingsGroup{{
			Matcher: ev.matcher,
			Hooks:   []settingsHook{{Type: "command", Command: q + " hook " + ev.name, Timeout: ev.timeout, Async: ev.async}},
		}}
	}
	// ordered by hand, so that the statusLine reads type, command, then the rest
	line := map[string]json.RawMessage{}
	line["type"] = json.RawMessage(`"command"`)
	cmd, _ := json.Marshal(q + " statusline")
	line["command"] = cmd
	if user != nil {
		for k, v := range user.Extras {
			line[k] = v
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(map[string]any{"hooks": hooks, "statusLine": line}); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// hasSettingsFlag says whether the user gave claude a --settings of their own.
func hasSettingsFlag(args []string) bool {
	for _, a := range args {
		if a == "--settings" || strings.HasPrefix(a, "--settings=") {
			return true
		}
	}
	return false
}
