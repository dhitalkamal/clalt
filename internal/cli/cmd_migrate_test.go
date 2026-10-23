package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/migrate"
)

// migrateHome points HOME, XDG_CONFIG_HOME and CLAUDE_CONFIG_DIR at a temp tree,
// so nothing in these tests can reach the real ~/.claude or ~/.config/clalt. it
// returns the profiles folder and the shared projects folder.
func migrateHome(t *testing.T) (profiles, shared string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	return config.ProfilesDir(), config.ClaudeProjectsDir()
}

func migrateWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runMigrate sends `clalt migrate ...` through the real dispatcher, keeping
// stdout and stderr apart.
func runMigrate(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	app, out, _ := newTestApp(t)
	errOut := &bytes.Buffer{}
	app.Err = errOut
	code = run(app, append([]string{"migrate"}, args...))
	return code, out.String(), errOut.String()
}

// mustSymlink skips the test only where the OS will not create a link (windows
// without developer mode). anywhere else a link that cannot be made is a broken
// setup, and skipping would hide it.
func mustSymlink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlinks not available: %v", err)
		}
		t.Fatalf("symlink: %v", err)
	}
}

// twoTranscripts puts two transcripts, 12 bytes in all, in the profile "work".
func twoTranscripts(t *testing.T, profiles string) {
	t.Helper()
	root := filepath.Join(profiles, "work", "projects", "-p")
	migrateWrite(t, filepath.Join(root, "a.jsonl"), "first")
	migrateWrite(t, filepath.Join(root, "b.jsonl"), "second!")
}

// lockOut removes every permission from path for the rest of the test. it
// reports false where that cannot work: windows has no such bits and root
// ignores them.
func lockOut(t *testing.T, path string) bool {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		return false
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	return true
}

func TestMigrateCopiesIntoTheSharedHistory(t *testing.T) {
	profiles, shared := migrateHome(t)
	twoTranscripts(t, profiles)

	code, out, errOut := runMigrate(t)
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	for rel, want := range map[string]string{"-p/a.jsonl": "first", "-p/b.jsonl": "second!"} {
		got, err := os.ReadFile(filepath.Join(shared, filepath.FromSlash(rel)))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q (%v), want %q", rel, got, err, want)
		}
	}
	for _, want := range []string{shared, "work", "copied 2 files (12 B)", "total: copied 2 files (12 B)", "left as they were"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestMigrateDryRunPrintsThePlanAndWritesNothing(t *testing.T) {
	profiles, shared := migrateHome(t)
	twoTranscripts(t, profiles)

	code, out, errOut := runMigrate(t, "--dry-run")
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	for _, want := range []string{"dry run", "would copy 2 files (12 B)", "nothing was written"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if _, err := os.Lstat(filepath.Dir(shared)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a dry run created %s: %v", filepath.Dir(shared), err)
	}
}

func TestMigrateIsRepeatable(t *testing.T) {
	profiles, _ := migrateHome(t)
	twoTranscripts(t, profiles)
	runMigrate(t)

	code, out, _ := runMigrate(t)
	if want := "copied 0 files (0 B), 2 already there"; code != 0 || !strings.Contains(out, want) {
		t.Errorf("second run exit %d, want %q in:\n%s", code, want, out)
	}
}

func TestMigrateWithoutProfileFolders(t *testing.T) {
	migrateHome(t)
	code, out, _ := runMigrate(t)
	if code != 0 || !strings.Contains(out, "no profile folders") {
		t.Errorf("exit %d, output %q", code, out)
	}
}

func TestMigrateJSON(t *testing.T) {
	profiles, shared := migrateHome(t)
	twoTranscripts(t, profiles)
	migrateWrite(t, filepath.Join(shared, "-p", "a.jsonl"), "different")

	code, out, errOut := runMigrate(t, "--dry-run", "--json")
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	var rep migrate.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("not one json document: %v\n%s", err, out)
	}
	if !rep.DryRun || rep.Dest != shared || rep.Source != config.ProfilesDir() {
		t.Errorf("report header wrong: %+v", rep)
	}
	if len(rep.Profiles) != 1 {
		t.Fatalf("profiles = %+v, want one", rep.Profiles)
	}
	p := rep.Profiles[0]
	if p.Name != "work" || p.Status != migrate.StatusFolder || p.Copied != 1 || p.Bytes != 7 ||
		!reflect.DeepEqual(p.Conflicts, []migrate.Conflict{{Path: "-p/a.jsonl", Reason: migrate.ReasonDiffers}}) {
		t.Errorf("profile = %+v", p)
	}
	if rep.Total.Copied != 1 || rep.Total.Conflicts != 1 {
		t.Errorf("total = %+v", rep.Total)
	}
	// empty lists stay [] so a consumer can loop without a nil check
	if !strings.Contains(out, `"failed": []`) {
		t.Errorf("failed should be an empty list:\n%s", out)
	}
}
