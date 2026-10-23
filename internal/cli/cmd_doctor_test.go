package cli

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

type doctorCheck struct {
	Scope, Check, Status, Detail string
}

type doctorOut struct {
	Checks []doctorCheck
	Failed int
	Warned int
}

// doctor runs `clalt doctor --json` and returns the exit code and the report.
func doctor(t *testing.T, e *testEnv) (int, doctorOut) {
	t.Helper()
	e.reset()
	code := e.run("doctor", "--json")
	var rep doctorOut
	if err := json.Unmarshal(e.out.Bytes(), &rep); err != nil {
		t.Fatalf("not json: %v\n%s\nstderr: %s", err, e.out.String(), e.errOut.String())
	}
	return code, rep
}

func (d doctorOut) find(scope, check string) (doctorCheck, bool) {
	for _, c := range d.Checks {
		if c.Scope == scope && c.Check == check {
			return c, true
		}
	}
	return doctorCheck{}, false
}

// want fails the test unless the check exists with this status and, when
// detail is not empty, mentions it.
func (d doctorOut) want(t *testing.T, scope, check, status, detail string) {
	t.Helper()
	c, ok := d.find(scope, check)
	if !ok {
		t.Errorf("no %s/%s check in %+v", scope, check, d.Checks)
		return
	}
	if c.Status != status || !strings.Contains(c.Detail, detail) {
		t.Errorf("%s/%s = %s %q, want %s mentioning %q", scope, check, c.Status, c.Detail, status, detail)
	}
}

// snapshot lists every path under dir with its size, mode and link target, so a
// test can tell that nothing was written, removed or touched.
func snapshot(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		line := path + "|" + info.Mode().String()
		if info.Mode().IsRegular() {
			body, _ := os.ReadFile(path)
			line += "|" + string(body)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			target, _ := os.Readlink(path)
			line += "|" + target
		}
		out = append(out, line)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func TestDoctorOnAHealthyInstall(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.at(theClock)
	e.signIn("work", "w@example.com")

	code, rep := doctor(t, e)

	if code != 0 || rep.Failed != 0 || rep.Warned != 0 {
		t.Fatalf("exit %d, %d failed, %d warned: %+v", code, rep.Failed, rep.Warned, rep.Checks)
	}
	rep.want(t, "claude", "version", "ok", "2.1.303")
	for _, check := range []string{"folder", "links", "mcp", "signedIn", "identity", "loginDue"} {
		rep.want(t, "work", check, "ok", "")
	}
	rep.want(t, "work", "identity", "ok", "w@example.com")
	rep.want(t, "work", "loginDue", "ok", dateOf(loginDue(theClock)))
	rep.want(t, "shell", "authEnv", "ok", "")
	rep.want(t, "settings", "authSettings", "ok", "")
	rep.want(t, "leftover", "oldVault", "ok", "")
	rep.want(t, "leftover", "oldProfiles", "ok", "")
}

func TestDoctorTextHasOneLinePerCheckAndASummary(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")

	code := e.run("doctor")

	out := e.out.String()
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" || strings.HasPrefix(line, "ok ") || strings.HasPrefix(line, "warn ") || strings.HasPrefix(line, "fail ") || strings.Contains(line, "all checks passed") {
			continue
		}
		t.Errorf("a line that is no check or summary: %q", line)
	}
	if !strings.Contains(out, "ok    work: signed in") || !strings.Contains(out, "all checks passed") {
		t.Errorf("output:\n%s", out)
	}
}

func TestDoctorNeverChangesAnything(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	// give it plenty to complain about
	folder := e.folder("work")
	os.Remove(filepath.Join(folder, "settings.json"))
	e.claude.signOut(folder)
	t.Setenv("ANTHROPIC_API_KEY", "sk-secret-value")
	for _, p := range []string{config.VaultDir(), filepath.Join(config.ProfilesDir(), "old", "projects", "-p")} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(config.ProfilesDir(), "old", "projects", "-p", "a.jsonl"), []byte("{}"), 0o644)
	os.WriteFile(config.VaultKeyPath(), []byte("not a real key"), 0o600)
	before := snapshot(t, e.home)

	doctor(t, e)

	after := snapshot(t, e.home)
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("doctor changed the tree:\nbefore %v\nafter  %v", before, after)
	}
}

func TestDoctorFailsWithoutClaude(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	t.Setenv("CLALT_CLAUDE_BIN", filepath.Join(e.home, "no-such-claude"))

	code, rep := doctor(t, e)

	rep.want(t, "claude", "version", "fail", "CLALT_CLAUDE_BIN")
	if code != 1 || rep.Failed == 0 {
		t.Errorf("exit %d, failed %d", code, rep.Failed)
	}
	// a claude that cannot be asked is not a reason to repeat the failure per account
	rep.want(t, "work", "signedIn", "warn", "claude")
}

func TestDoctorChecksTheClaudeVersionAgainstTheMinimum(t *testing.T) {
	cases := []struct {
		version, status string
	}{
		{"2.1.300", "fail"},
		{"2.0.999", "fail"},
		{"2.1.301", "ok"},
		{"2.1.1000", "ok"},
		{"3.0.0", "ok"},
	}
	for _, c := range cases {
		t.Run(c.version, func(t *testing.T) {
			e := newEnv(t)
			e.claude.set("VERSION", c.version)

			code, rep := doctor(t, e)

			rep.want(t, "claude", "version", c.status, c.version)
			if (c.status == "fail") != (code == 1) {
				t.Errorf("exit code %d for %s", code, c.status)
			}
		})
	}
}

func TestDoctorWarnsWhenAnAccountFolderIsMissing(t *testing.T) {
	e := newEnv(t)
	if _, err := e.app.createAccount("work"); err != nil {
		t.Fatal(err)
	}

	code, rep := doctor(t, e)

	rep.want(t, "work", "folder", "warn", "clalt login work")
	if code != 0 {
		t.Errorf("exit %d, want 0: signing in fixes it", code)
	}
}

func TestDoctorReportsLinksThatAreOutOfSyncWithoutFixingThem(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	settings := filepath.Join(e.folder("work"), "settings.json")
	if err := os.Remove(settings); err != nil {
		t.Fatal(err)
	}

	code, rep := doctor(t, e)

	rep.want(t, "work", "links", "warn", "clalt run work")
	rep.want(t, "work", "links", "warn", "settings.json")
	if code != 0 {
		t.Errorf("a link that run repairs is a warning, exit %d", code)
	}
	if exists(settings) {
		t.Error("doctor repaired the link")
	}
}

func TestDoctorFailsWhenProjectsIsARealFolder(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	projects := filepath.Join(e.folder("work"), "projects")
	os.Remove(projects)
	if err := os.MkdirAll(filepath.Join(projects, "-p"), 0o755); err != nil {
		t.Fatal(err)
	}

	code, rep := doctor(t, e)

	rep.want(t, "work", "links", "fail", "history")
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
}

func TestDoctorReportsMCPServersOutOfSync(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	shared := `{"mcpServers":{"github":{"command":"gh-mcp"},"linear":{"command":"secret-command --token=abc123"}}}`
	if err := os.WriteFile(filepath.Join(e.home, ".claude.json"), []byte(shared), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(e.folder("work"), ".claude.json"))

	_, rep := doctor(t, e)

	rep.want(t, "work", "mcp", "warn", "linear")
	if strings.Contains(e.out.String(), "abc123") || strings.Contains(e.out.String(), "secret-command") {
		t.Error("a server definition leaked into the report")
	}
	if after, _ := os.ReadFile(filepath.Join(e.folder("work"), ".claude.json")); string(after) != string(before) {
		t.Error("doctor synced the servers")
	}
}

func TestDoctorTakesNoArguments(t *testing.T) {
	e := newEnv(t)
	if code := e.run("doctor", "work"); code != 2 || !strings.Contains(e.errOut.String(), "usage: clalt doctor") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}
