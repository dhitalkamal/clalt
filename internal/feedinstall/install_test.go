package feedinstall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var events = []Event{
	{Name: "SessionStart", Timeout: 5},
	{Name: "Notification", Matcher: "permission_prompt", Timeout: 5},
	{Name: "PostToolUse", Async: true},
}

var theTime = time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)

// the user's file, written the way people write them: their own indent and key order
const userFile = `{
    "model": "opus",
    "hooks": {
        "Stop": [{"hooks": [{"type": "command", "command": "say done"}]}],
        "SessionStart": [{"matcher": "startup", "hooks": [{"type": "command", "command": "echo hi"}]}]
    },
    "statusLine": {"type": "command", "command": "~/bin/line.sh", "padding": 1}
}
`

type rig struct {
	t        *testing.T
	settings string
	opts     Options
}

func newRig(t *testing.T, body string) *rig {
	t.Helper()
	dir := t.TempDir()
	r := &rig{t: t, settings: filepath.Join(dir, "claude", "settings.json")}
	if err := os.MkdirAll(filepath.Dir(r.settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if body != "" {
		if err := os.WriteFile(r.settings, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r.opts = Options{
		Settings: r.settings, BackupDir: filepath.Join(dir, "clalt", "backups", "settings"),
		StatePath: filepath.Join(dir, "clalt", "feed-install.json"),
		Bin:       "/opt/clalt", Events: events, Now: theTime,
	}
	return r
}

func (r *rig) read() string {
	r.t.Helper()
	b, err := os.ReadFile(r.settings)
	if err != nil {
		r.t.Fatal(err)
	}
	return string(b)
}

func (r *rig) install() Plan {
	r.t.Helper()
	p, err := Prepare(r.opts)
	if err != nil {
		r.t.Fatal(err)
	}
	if err := p.Apply(); err != nil {
		r.t.Fatal(err)
	}
	return p
}

func parse(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("%v: %s", err, s)
	}
	return m
}

func commands(t *testing.T, s, event string) []string {
	t.Helper()
	var out []string
	hooks, _ := parse(t, s)["hooks"].(map[string]any)
	for _, g := range hooks[event].([]any) {
		for _, h := range g.(map[string]any)["hooks"].([]any) {
			out = append(out, h.(map[string]any)["command"].(string))
		}
	}
	return out
}

func TestInstallMergesAndKeepsEverythingTheUserHas(t *testing.T) {
	r := newRig(t, userFile)

	p := r.install()

	got := r.read()
	m := parse(t, got)
	if m["model"] != "opus" {
		t.Errorf("model lost: %v", m["model"])
	}
	if c := commands(t, got, "Stop"); len(c) != 1 || c[0] != "say done" {
		t.Errorf("the user's Stop hook changed: %v", c)
	}
	// the user's handler first, then ours
	c := commands(t, got, "SessionStart")
	if len(c) != 2 || c[0] != "echo hi" || c[1] != "[ -x '/opt/clalt' ] || exit 0; exec '/opt/clalt' hook SessionStart clalt-feed" {
		t.Errorf("SessionStart: %v", c)
	}
	if c := commands(t, got, "PostToolUse"); len(c) != 1 || !strings.HasSuffix(c[0], "clalt-feed") {
		t.Errorf("PostToolUse: %v", c)
	}
	line := m["statusLine"].(map[string]any)
	if !strings.HasSuffix(line["command"].(string), "# clalt-feed") || !strings.Contains(line["command"].(string), "exec '/opt/clalt' statusline") || line["padding"] != float64(1) {
		t.Errorf("statusLine: %v", line)
	}
	if !p.Changed {
		t.Error("not marked changed")
	}
}

func TestInstallWritesABackupOfTheOldFileAndKeepsTheStatusLineInOurState(t *testing.T) {
	r := newRig(t, userFile)

	p := r.install()

	b, err := os.ReadFile(p.BackupPath)
	if err != nil || string(b) != userFile {
		t.Errorf("backup %q: %v", b, err)
	}
	if !strings.HasPrefix(p.BackupPath, r.opts.BackupDir) || !strings.Contains(p.BackupPath, "20261023T090000Z") {
		t.Errorf("backup path %s", p.BackupPath)
	}
	orig, ok := LoadOriginal(r.opts.StatePath)
	if !ok || !strings.Contains(string(orig), "line.sh") {
		t.Errorf("state original: %s %v", orig, ok)
	}
	if fi, _ := os.Stat(p.BackupPath); fi.Mode().Perm() != 0o600 && !isWindows() {
		t.Errorf("backup mode %v", fi.Mode())
	}
}

func TestInstallTwiceChangesNothingAndMakesNoSecondBackup(t *testing.T) {
	r := newRig(t, userFile)
	r.install()
	after := r.read()

	p, err := Prepare(r.opts)

	if err != nil || p.Changed {
		t.Fatalf("changed %v err %v", p.Changed, err)
	}
	if err := p.Apply(); err != nil {
		t.Fatal(err)
	}
	if r.read() != after {
		t.Error("the file changed")
	}
	if ents, _ := os.ReadDir(r.opts.BackupDir); len(ents) != 1 {
		t.Errorf("backups: %d", len(ents))
	}
	if orig, ok := LoadOriginal(r.opts.StatePath); !ok || !strings.Contains(string(orig), "line.sh") {
		t.Error("state lost the original")
	}
}

func TestUninstallOfAnUntouchedFileRestoresTheUsersBytes(t *testing.T) {
	r := newRig(t, userFile)
	r.install()

	res, err := Uninstall(r.opts)

	if err != nil || !res.Restored {
		t.Fatalf("%+v %v", res, err)
	}
	if r.read() != userFile {
		t.Errorf("not byte for byte:\n%s", r.read())
	}
	if _, err := os.Stat(r.opts.StatePath); err == nil {
		t.Error("state left behind")
	}
}

func TestUninstallAfterTheUserEditedRemovesOnlyOurEntries(t *testing.T) {
	r := newRig(t, userFile)
	r.install()
	edited := strings.Replace(r.read(), `"model": "opus"`, `"model": "sonnet", "theme": "dark"`, 1)
	if err := os.WriteFile(r.settings, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := Uninstall(r.opts)

	if err != nil || res.Restored {
		t.Fatalf("%+v %v", res, err)
	}
	m := parse(t, r.read())
	if m["model"] != "sonnet" || m["theme"] != "dark" {
		t.Errorf("user edits lost: %v", m)
	}
	if strings.Contains(r.read(), "clalt-feed") {
		t.Error("ours left")
	}
	if c := commands(t, r.read(), "SessionStart"); len(c) != 1 || c[0] != "echo hi" {
		t.Errorf("SessionStart: %v", c)
	}
	if _, ok := m["hooks"].(map[string]any)["Notification"]; ok {
		t.Error("an empty event array was left")
	}
	if line := m["statusLine"].(map[string]any); line["command"] != "~/bin/line.sh" || line["padding"] != float64(1) {
		t.Errorf("statusLine not restored: %v", line)
	}
	if ents, _ := os.ReadDir(r.opts.BackupDir); len(ents) != 2 {
		t.Errorf("the edited file was not backed up: %d", len(ents))
	}
}

func TestInstallOnAMissingFileCreatesItAndUninstallRemovesIt(t *testing.T) {
	r := newRig(t, "")

	p := r.install()

	if p.BackupPath != "" {
		t.Errorf("backup of nothing: %s", p.BackupPath)
	}
	m := parse(t, r.read())
	if m["statusLine"] == nil || m["hooks"] == nil {
		t.Errorf("%v", m)
	}
	if _, err := Uninstall(r.opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(r.settings); err == nil {
		t.Error("the file we created is still there")
	}
}

func TestInstallWithNoStatusLineAddsOursAndUninstallDeletesIt(t *testing.T) {
	r := newRig(t, `{"model":"opus"}`+"\n")
	r.install()
	if _, ok := LoadOriginal(r.opts.StatePath); ok {
		t.Error("an original from nothing")
	}
	edited := strings.Replace(r.read(), `"opus"`, `"haiku"`, 1)
	os.WriteFile(r.settings, []byte(edited), 0o600)

	if _, err := Uninstall(r.opts); err != nil {
		t.Fatal(err)
	}

	m := parse(t, r.read())
	if _, ok := m["statusLine"]; ok || m["model"] != "haiku" || m["hooks"] != nil {
		t.Errorf("%v", m)
	}
}

func TestStatusReadsTheFileNotTheState(t *testing.T) {
	r := newRig(t, userFile)
	if s, err := Check(r.opts); err != nil || s.Installed {
		t.Fatalf("before: %+v %v", s, err)
	}
	r.install()

	s, err := Check(r.opts)

	if err != nil || !s.Installed || s.Hooks != len(events) || !s.StatusLine || !s.Chained || s.Backup == "" {
		t.Errorf("%+v %v", s, err)
	}
	os.Remove(r.opts.StatePath)
	if s, _ := Check(r.opts); !s.Installed {
		t.Error("lost without the state file")
	}
}

func TestAHandlerOfAnotherClaltWithTheMarkerIsStillFoundByUninstall(t *testing.T) {
	r := newRig(t, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"'/old/clalt' hook Stop clalt-feed"},{"type":"command","command":"keep"}]}]}}`)

	if _, err := Uninstall(r.opts); err != nil {
		t.Fatal(err)
	}

	if c := commands(t, r.read(), "Stop"); len(c) != 1 || c[0] != "keep" {
		t.Errorf("%v", c)
	}
}
