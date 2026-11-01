package office

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhitalkamal/clalt/internal/fsutil"
	"github.com/dhitalkamal/clalt/internal/office/layout"
)

const (
	idlePath    = "/api/idle"
	idleFile    = "idle.json"
	idleMaxKeys = 200
)

// IdleRules is where an idle agent goes: a choice per account, team member
// and subagent type, with a default for sessions and one for the team.
type IdleRules struct {
	Default     string            `json:"default"`
	TeamDefault string            `json:"teamDefault"`
	Accounts    map[string]string `json:"accounts"`
	Teams       map[string]string `json:"teams"`
	Subtypes    map[string]string `json:"subtypes"`
}

func defaultIdle() IdleRules {
	return IdleRules{
		Default: "break", TeamDefault: "desk",
		Accounts: map[string]string{}, Teams: map[string]string{}, Subtypes: map[string]string{},
	}
}

// validIdleChoice allows desk, break, hide and room:<id>. the room is not
// checked against a layout, the page falls back when it is gone.
func validIdleChoice(c string) bool {
	if c == "desk" || c == "break" || c == "hide" {
		return true
	}
	id, ok := strings.CutPrefix(c, "room:")
	return ok && layout.ValidID(id)
}

func validIdleKey(k string) bool {
	if k == "" || len(k) > 64 {
		return false
	}
	for _, r := range k {
		if r < 0x20 || r > 0x7e {
			return false
		}
	}
	return true
}

// check returns the first problem of the rules as a one-line message.
func (r *IdleRules) check() error {
	if !validIdleChoice(r.Default) {
		return errors.New(`"default" must be desk, break, hide or room:<id>`)
	}
	if !validIdleChoice(r.TeamDefault) {
		return errors.New(`"teamDefault" must be desk, break, hide or room:<id>`)
	}
	maps := []struct {
		name  string
		m     map[string]string
		valid func(string) bool
	}{
		{"accounts", r.Accounts, layout.ValidAccount},
		{"teams", r.Teams, validIdleKey},
		{"subtypes", r.Subtypes, validIdleKey},
	}
	for _, mp := range maps {
		if len(mp.m) > idleMaxKeys {
			return fmt.Errorf("%q has more than %d entries", mp.name, idleMaxKeys)
		}
		for k, c := range mp.m {
			if !mp.valid(k) {
				return fmt.Errorf("%q has a key that is not allowed", mp.name)
			}
			if !validIdleChoice(c) {
				return fmt.Errorf("%q has a choice that is not desk, break, hide or room:<id>", mp.name)
			}
		}
	}
	return nil
}

// parseIdle decodes and checks a body: no unknown fields, both defaults set.
func parseIdle(raw []byte) (IdleRules, error) {
	var r IdleRules
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil || dec.More() {
		return r, errors.New("body must be the idle rules as JSON")
	}
	if err := r.check(); err != nil {
		return r, err
	}
	if r.Accounts == nil {
		r.Accounts = map[string]string{}
	}
	if r.Teams == nil {
		r.Teams = map[string]string{}
	}
	if r.Subtypes == nil {
		r.Subtypes = map[string]string{}
	}
	return r, nil
}

// idlePathOn is the file, empty when there is no store to keep it in.
func (h *handler) idlePathOn() string {
	if h.cfg.Layouts == nil || h.cfg.Layouts.Root == "" {
		return ""
	}
	return filepath.Join(h.cfg.Layouts.Root, idleFile)
}

// idleGet is the read route. a missing, unreadable or invalid file answers
// the defaults, the last two are logged. the ETag is the version of the file.
func (h *handler) idleGet(w http.ResponseWriter) {
	p := h.idlePathOn()
	if p == "" {
		h.writeJSON(w, defaultIdle())
		return
	}
	raw, err := fsutil.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		setVersion(w, noVersion)
		h.writeJSON(w, defaultIdle())
		return
	}
	if err == nil {
		setVersion(w, versionOf(raw))
		var r IdleRules
		if r, err = parseIdle(raw); err == nil {
			h.writeJSON(w, r)
			return
		}
	}
	h.logf("idle choices: %s ignored: %v", idleFile, err)
	h.writeJSON(w, defaultIdle())
}

// idleCurrent reads the file as stored: the rules of it (defaults when it is
// missing or invalid) and its version.
func (h *handler) idleCurrent(p string) (IdleRules, string, error) {
	raw, err := fsutil.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return defaultIdle(), noVersion, nil
	}
	if err != nil {
		return IdleRules{}, "", err
	}
	r, perr := parseIdle(raw)
	if perr != nil {
		r = defaultIdle()
	}
	return r, versionOf(raw), nil
}

// idleSave is the write route, after the guards of editRoute. idleMu holds
// the read, the version check and the write together.
func (h *handler) idleSave(w http.ResponseWriter, r *http.Request, body []byte) {
	p := h.idlePathOn()
	if p == "" {
		http.Error(w, "idle choices are not available", http.StatusNotFound)
		return
	}
	rules, err := parseIdle(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.idleMu.Lock()
	defer h.idleMu.Unlock()
	cur, ver, err := h.idleCurrent(p)
	if err != nil {
		h.logf("idle choices read failed: %v", err)
		http.Error(w, "could not read the idle choices", http.StatusInternalServerError)
		return
	}
	if staleWrite(r, ver) {
		setVersion(w, ver)
		h.conflict(w, map[string]any{"error": "the idle choices changed since they were loaded", "rules": cur, "version": ver})
		return
	}
	raw, err := json.Marshal(rules)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(p), 0o700)
	}
	if err == nil {
		err = fsutil.WriteFileAtomic(p, raw, 0o600)
	}
	if err != nil {
		h.logf("idle choices save failed: %v", err)
		http.Error(w, "could not save the idle choices", http.StatusInternalServerError)
		return
	}
	setVersion(w, versionOf(raw))
	h.writeJSON(w, rules)
}
