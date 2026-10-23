// Package feedinstall adds clalt's hooks and statusline to the settings.json of
// the shared claude home, so that plain claude feeds clalt too, and takes them
// out again. it edits one file, keeps every key and handler that is not its own,
// and finds its own by a marker in the command.
package feedinstall

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Marker is the last word of every command clalt puts in settings.json. the
// hook and the statusline ignore it; install, uninstall and status find clalt's
// entries by it.
const Marker = "clalt-feed"

// Event is one hook event to install, see cli.hookEvents.
type Event struct {
	Name    string
	Matcher string
	Timeout int
	Async   bool
}

type object = map[string]json.RawMessage

// merged is the result of Merge.
type merged struct {
	Out []byte
	// Original is the user's statusLine that ours replaced, nil when there was
	// none or it was clalt's already
	Original json.RawMessage
	Hooks    int  // handlers added
	Line     bool // the statusLine was set to clalt's
}

func (m merged) changed() bool { return m.Hooks > 0 || m.Line }

// IsOurs says whether a settings command is one that install put there.
func IsOurs(cmd string) bool { return hasMarker(cmd) }

// hasMarker says whether a command is one of clalt's.
func hasMarker(cmd string) bool {
	f := strings.Fields(cmd)
	return len(f) > 0 && f[len(f)-1] == Marker
}

func parseObject(raw []byte) (object, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, errors.New("the file is empty, which is not valid JSON")
	}
	var o object
	if err := json.Unmarshal(raw, &o); err != nil || o == nil {
		return nil, errors.New("not valid JSON, or not a JSON object")
	}
	return o, nil
}

// encode writes o with sorted keys and two space indent, no HTML escaping, a
// trailing newline. values that were raw keep what they say.
func encode(o object) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(o); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// marshal is json.Marshal without the HTML escaping, so that && in a command
// stays as the user wrote it.
func marshal(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buf.Bytes()), nil
}

type handler struct {
	Command string `json:"command"`
}

// groupHasMarker says whether a matcher group holds a handler of clalt's.
func groupHasMarker(g json.RawMessage) bool {
	var grp struct {
		Hooks []handler `json:"hooks"`
	}
	if json.Unmarshal(g, &grp) != nil {
		return false
	}
	for _, h := range grp.Hooks {
		if hasMarker(h.Command) {
			return true
		}
	}
	return false
}

func hooksOf(o object) (object, error) {
	h := object{}
	if raw, ok := o["hooks"]; ok {
		if err := json.Unmarshal(raw, &h); err != nil || h == nil {
			return nil, errors.New(`"hooks" is not an object`)
		}
	}
	return h, nil
}

func groupsOf(h object, event string) ([]json.RawMessage, error) {
	var groups []json.RawMessage
	if raw, ok := h[event]; ok {
		if err := json.Unmarshal(raw, &groups); err != nil {
			return nil, fmt.Errorf("hooks.%s is not a list", event)
		}
	}
	return groups, nil
}

// guarded is the start of a hook command: nothing at all when clalt is gone
// (moved, uninstalled, a build that was cleaned), instead of a "not found" on
// every event, and exec otherwise so that clalt's exit code and stdin are claude's.
func guarded(quotedBin string) string {
	return "[ -x " + quotedBin + " ] || exit 0; exec " + quotedBin
}

// Merge adds clalt's handlers and statusLine to the settings in raw (nil or
// empty is a file that does not exist yet). q quotes the path of bin.
func merge(raw []byte, bin string, quote func(string) string, events []Event) (merged, error) {
	var o object
	if len(bytes.TrimSpace(raw)) == 0 && raw == nil {
		o = object{}
	} else {
		var err error
		if o, err = parseObject(raw); err != nil {
			return merged{}, err
		}
	}
	var res merged
	hooks, err := hooksOf(o)
	if err != nil {
		return res, err
	}
	for _, ev := range events {
		groups, err := groupsOf(hooks, ev.Name)
		if err != nil {
			return res, err
		}
		if hasOurs(groups) {
			continue
		}
		g, _ := marshal(group{Matcher: ev.Matcher, Hooks: []hookDef{{
			Type: "command", Command: guarded(quote(bin)) + " hook " + ev.Name + " " + Marker, Timeout: ev.Timeout, Async: ev.Async,
		}}})
		groups = append(groups, g)
		hooks[ev.Name], _ = marshal(groups)
		res.Hooks++
	}
	if res.Hooks > 0 {
		o["hooks"], _ = marshal(hooks)
	}
	if err := setStatusLine(o, quote(bin), &res); err != nil {
		return res, err
	}
	if !res.changed() {
		res.Out = raw
		return res, nil
	}
	res.Out, err = encode(o)
	return res, err
}

func hasOurs(groups []json.RawMessage) bool {
	for _, g := range groups {
		if groupHasMarker(g) {
			return true
		}
	}
	return false
}

type hookDef struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
	Async   bool   `json:"async,omitempty"`
}

type group struct {
	Matcher string    `json:"matcher,omitempty"`
	Hooks   []hookDef `json:"hooks"`
}

// statusCommand is the statusLine command. clalt missing must not blank the
// line: then the user's own command, the one it replaced, runs by itself. that
// is the one place a copy of it is in settings.json. the marker is a comment at
// the end, which is where uninstall looks for it.
func statusCommand(quotedBin, user string) string {
	run := "exec " + quotedBin + " statusline " + Marker
	if user == "" {
		return "[ -x " + quotedBin + " ] || exit 0; " + run + " # " + Marker
	}
	return "if [ -x " + quotedBin + " ]; then " + run + "; else\n" + user + "\n\nfi # " + Marker
}

// setStatusLine puts clalt's statusLine in place of a command statusLine of the
// user's, whose raw object it hands back. a statusLine of another kind is left
// alone: clalt could not run it, and replacing it would lose it.
func setStatusLine(o object, quotedBin string, res *merged) error {
	line := object{}
	userCommand := ""
	var user object
	if raw, ok := o["statusLine"]; ok {
		if json.Unmarshal(raw, &user) != nil || user == nil {
			return nil
		}
		var kind, cmd string
		_ = json.Unmarshal(user["type"], &kind)
		_ = json.Unmarshal(user["command"], &cmd)
		if hasMarker(cmd) || kind != "command" {
			return nil
		}
		res.Original = raw
		userCommand = cmd
		for _, k := range []string{"padding", "refreshInterval", "hideVimModeIndicator"} {
			if v, ok := user[k]; ok {
				line[k] = v
			}
		}
	}
	line["type"] = json.RawMessage(`"command"`)
	line["command"], _ = marshal(statusCommand(quotedBin, userCommand))
	o["statusLine"], _ = marshal(line)
	res.Line = true
	return nil
}
