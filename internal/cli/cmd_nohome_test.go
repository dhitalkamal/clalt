package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// an app as it starts when HOME is not set, in a working directory that holds a
// state of somebody else's where a relative path would find it
func appWithoutAHome(t *testing.T) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	newEnv(t)
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	cwd := t.TempDir()
	t.Chdir(cwd)
	foreign := filepath.Join(".config", "clalt")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "state.json"), []byte(`{"accounts":{"stranger":{"addedAt":"2026-01-01T00:00:00Z"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	app.Out, app.Err, app.In = out, errOut, strings.NewReader("")
	return app, out, errOut
}

func TestCommandsThatNeedStateRefuseWithoutAHomeAndLeaveTheWorkingDirectoryAlone(t *testing.T) {
	for _, args := range [][]string{
		{"list"}, {"login", "fresh"}, {"logout", "stranger"}, {"remove", "stranger", "--force"},
		{"rename", "stranger", "x"}, {"run", "stranger"}, {"budget"}, {"usage"}, {"sessions"},
		{"migrate"}, {"dashboard"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app, out, errOut := appWithoutAHome(t)
			before := snapshot(t, ".")

			code := run(app, args)

			if code != 1 || !strings.Contains(errOut.String(), "HOME") {
				t.Errorf("exit %d, stdout %q, stderr %q: want 1 and a message about HOME", code, out.String(), errOut.String())
			}
			if strings.Contains(out.String(), "stranger") {
				t.Errorf("it read the state in the working directory:\n%s", out.String())
			}
			if after := snapshot(t, "."); strings.Join(after, "\n") != strings.Join(before, "\n") {
				t.Errorf("the working directory changed:\n%v\nthen\n%v", before, after)
			}
		})
	}
}

// what needs no state keeps working
func TestVersionAndHelpWorkWithoutAHome(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"help"}, {"--version"}, {"whoami"}} {
		app, out, errOut := appWithoutAHome(t)

		if code := run(app, args); code != 0 || out.Len() == 0 {
			t.Errorf("%v: exit %d, stdout %q, stderr %q", args, code, out.String(), errOut.String())
		}
	}
}

// doctor with no home: what it would look for is in places that do not exist
// for it, and a relative path is the working directory. each leftover check
// says that it could not look, and none of them reports what happens to be in
// the working directory instead.
func TestDoctorLeftoverChecksSayTheHomeIsUnknownAndDoNotReadTheWorkingDirectory(t *testing.T) {
	app, out, errOut := appWithoutAHome(t)
	for _, path := range []string{
		filepath.Join(".config", "clalt", "vault", "x"),
		filepath.Join(".config", "clalt", "vault.key"),
		filepath.Join(".config", "clalt", "keepalive.auto"),
		filepath.Join(".config", "clalt", "keepalive.disabled"),
		filepath.Join(".config", "clalt", "profiles", "stranger", "projects", "-p", "a.jsonl"),
		filepath.Join(".config", "systemd", "user", keepaliveLabel+".timer"),
		filepath.Join("Library", "LaunchAgents", keepaliveLabel+".plist"),
		filepath.Join(".claude", ".credentials.json"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	run(app, []string{"doctor", "--json"})

	var rep doctorOut
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("not json: %v\n%s\n%s", err, out.String(), errOut.String())
	}
	var leftovers int
	for _, c := range rep.Checks {
		if c.Scope != "leftover" {
			continue
		}
		leftovers++
		if c.Status != "warn" || !strings.Contains(c.Detail, "not checked") || !strings.Contains(c.Detail, "HOME") {
			t.Errorf("%s = %s %q, want a warning that it was not checked because HOME is not set", c.Check, c.Status, c.Detail)
		}
		for _, planted := range []string{"vault", "keepalive.", "profiles", "LaunchAgents", "systemd", ".credentials.json is there"} {
			if strings.Contains(c.Detail, planted) {
				t.Errorf("%s reports what is in the working directory: %q", c.Check, c.Detail)
			}
		}
	}
	if leftovers < 3 {
		t.Errorf("only %d leftover checks: %+v", leftovers, rep.Checks)
	}
}

// clalt's own folder can be known through XDG_CONFIG_HOME when the home is not.
// what is under clalt's folder is then checked, and what needs the home is not,
// and the reason is the home and not a run session
func TestDoctorChecksClaltsFolderButNotTheHomeWhenOnlyXDGIsSet(t *testing.T) {
	newEnv(t)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join(xdg, "clalt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "clalt", "keepalive.auto"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(xdg, "clalt", "profiles", "old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "clalt", "profiles", "old", ".credentials.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	app.Out, app.Err, app.In = out, errOut, strings.NewReader("")

	run(app, []string{"doctor", "--json"})

	var rep doctorOut
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("not json: %v\n%s\n%s", err, out.String(), errOut.String())
	}
	rep.want(t, "leftover", "oldKeepaliveMarkers", "warn", "keepalive.auto")
	rep.want(t, "leftover", "oldVault", "ok", "")
	// the folders and their login files are in clalt's own folder, so they are
	// found. the history needs the home, and that is what is not checked
	rep.want(t, "leftover", "oldProfiles", "warn", "HOME")
	rep.want(t, "leftover", "oldProfiles", "warn", filepath.Join("old", ".credentials.json"))
	if c, _ := rep.find("leftover", "oldProfiles"); strings.Contains(c.Detail, "run' session") {
		t.Errorf("oldProfiles blames a run session: %q", c.Detail)
	}
}
