package feedinstall

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// the installed commands are run by sh in these tests, the way claude does on unix
func runSh(t *testing.T, command, stdin string) (string, int) {
	t.Helper()
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return string(out), code
}

func installedCommands(t *testing.T, r *rig) (hook, line string) {
	t.Helper()
	r.install()
	m := parse(t, r.read())
	return commands(t, r.read(), "SessionStart")[0], m["statusLine"].(map[string]any)["command"].(string)
}

func TestAMissingClaltMakesTheHookASilentNoOp(t *testing.T) {
	if isWindows() {
		t.Skip("sh commands")
	}
	r := newRig(t, "")
	r.opts.Bin = filepath.Join(t.TempDir(), "gone", "clalt")
	hook, _ := installedCommands(t, r)

	out, code := runSh(t, hook, `{"session_id":"x"}`)

	if out != "" || code != 0 {
		t.Errorf("out %q exit %d", out, code)
	}
}

func TestAnInstalledHookRunsClaltWithItsInput(t *testing.T) {
	if isWindows() {
		t.Skip("sh commands")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "clalt")
	os.WriteFile(bin, []byte("#!/bin/sh\necho \"ran $*\"; cat\n"), 0o755)
	r := newRig(t, "")
	r.opts.Bin = bin
	hook, _ := installedCommands(t, r)

	out, code := runSh(t, hook, "in")

	if code != 0 || out != "ran hook SessionStart clalt-feed\nin" {
		t.Errorf("out %q exit %d", out, code)
	}
}

func TestAMissingClaltMakesTheStatusLineRunTheUsersOwnCommand(t *testing.T) {
	if isWindows() {
		t.Skip("sh commands")
	}
	r := newRig(t, `{"statusLine":{"type":"command","command":"cat; echo mine"}}`)
	r.opts.Bin = filepath.Join(t.TempDir(), "gone", "clalt")
	_, line := installedCommands(t, r)

	out, code := runSh(t, line, "payload ")

	if out != "payload mine\n" || code != 0 {
		t.Errorf("out %q exit %d", out, code)
	}
}

func TestAMissingClaltAndNoStatusLineOfTheUsersPrintsNothing(t *testing.T) {
	if isWindows() {
		t.Skip("sh commands")
	}
	r := newRig(t, "")
	r.opts.Bin = filepath.Join(t.TempDir(), "gone", "clalt")
	_, line := installedCommands(t, r)

	if out, code := runSh(t, line, "x"); out != "" || code != 0 {
		t.Errorf("out %q exit %d", out, code)
	}
}

func TestUninstallStillFindsGuardedCommands(t *testing.T) {
	if isWindows() {
		t.Skip("sh commands")
	}
	r := newRig(t, userFile)
	r.install()
	edited := strings.Replace(r.read(), `"opus"`, `"haiku"`, 1)
	os.WriteFile(r.settings, []byte(edited), 0o600)

	if _, err := Uninstall(r.opts); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.read(), "clalt-feed") {
		t.Error("ours left")
	}
}

func TestCheckReportsTheRecordedBinaryAndWhetherItIsStillThere(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "clalt")
	os.WriteFile(bin, []byte("x"), 0o755)
	r := newRig(t, "")
	r.opts.Bin = bin
	r.install()

	if s, _ := Check(r.opts); s.Bin != bin || s.BinMissing {
		t.Errorf("%+v", s)
	}
	os.Remove(bin)
	if s, _ := Check(r.opts); !s.BinMissing {
		t.Errorf("%+v", s)
	}
}

func TestAUserCommandEndingInABackslashOrAComment(t *testing.T) {
	if isWindows() {
		t.Skip("sh commands")
	}
	for name, user := range map[string]string{
		"backslash":   "echo mine \\",
		"comment":     "echo mine # trailing",
		"heredoc-ish": "echo mine;",
	} {
		t.Run(name, func(t *testing.T) {
			body, _ := marshal(map[string]any{"statusLine": map[string]any{"type": "command", "command": user}})
			r := newRig(t, string(body))
			r.opts.Bin = filepath.Join(t.TempDir(), "gone", "clalt")
			_, line := installedCommands(t, r)

			out, code := runSh(t, line, "")

			if code != 0 || !strings.HasPrefix(out, "mine") {
				t.Errorf("out %q exit %d in %q", out, code, line)
			}
		})
	}
}

func TestPrepareRefusesAStatusLineThatDoesNotParse(t *testing.T) {
	if isWindows() {
		t.Skip("sh commands")
	}
	// an unterminated quote cannot be saved by any joining
	body, _ := marshal(map[string]any{"statusLine": map[string]any{"type": "command", "command": "echo 'open"}})
	r := newRig(t, string(body))

	_, err := Prepare(r.opts)

	if err == nil || !strings.Contains(err.Error(), "does not parse") {
		t.Errorf("%v", err)
	}
	if r.read() != string(body) {
		t.Error("changed")
	}
}
