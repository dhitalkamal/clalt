package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// treeNames lists every path under root, so a test can prove nothing was added.
func treeNames(t *testing.T, root string) []string {
	t.Helper()
	var names []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil {
			names = append(names, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return names
}

// refusalText is exactly what a refused run prints to stderr.
func refusalText(dir string) string {
	return "error: CLAUDE_CONFIG_DIR points at an account folder (" + clean(dir) + "), not at the shared history. " +
		"run 'clalt migrate' from a normal shell, not inside a 'clalt run' session\n"
}

func wantRefusal(t *testing.T, code int, out, errOut, dir string) {
	t.Helper()
	if code != 1 || out != "" || errOut != refusalText(dir) {
		t.Errorf("exit %d, stdout %q, stderr %q, want exit 1, no report and %q", code, out, errOut, refusalText(dir))
	}
}

// inside a `clalt run` session CLAUDE_CONFIG_DIR is an account folder, and the
// usual lookup would then aim the copy at that account's own projects.
func TestMigrateRefusesInsideARunSession(t *testing.T) {
	profiles, shared := migrateHome(t)
	twoTranscripts(t, profiles)
	account := filepath.Join(config.ProfilesDir(), "work")
	dirs := map[string]string{
		"the account folder": account,
		"below it":           filepath.Join(account, "nested"),
		"the clalt folder":   config.ClaltDir(),
	}
	dirs["through a link"] = aliasOf(t, account)
	before := treeNames(t, config.ClaltDir())

	for name, dir := range dirs {
		t.Run(name, func(t *testing.T) {
			t.Setenv("CLAUDE_CONFIG_DIR", dir)
			code, out, errOut := runMigrate(t)
			wantRefusal(t, code, out, errOut, dir)
		})
	}
	if got := treeNames(t, config.ClaltDir()); strings.Join(got, "\n") != strings.Join(before, "\n") {
		t.Errorf("a refused run changed the clalt folder:\n  was %v\n  now %v", before, got)
	}
	if _, err := os.Lstat(shared); err == nil {
		t.Errorf("a refused run created %s", shared)
	}
}

func TestMigrateAllowsAClaudeConfigDirOutsideClalt(t *testing.T) {
	profiles, defaultShared := migrateHome(t)
	twoTranscripts(t, profiles)
	custom := filepath.Join(t.TempDir(), "my-claude")
	t.Setenv("CLAUDE_CONFIG_DIR", custom)

	code, out, errOut := runMigrate(t)
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	mustHave(t, out, "migrating profile history into "+filepath.Join(custom, "projects"), "work  copied 2 files (12 B)")
	if got, err := os.ReadFile(filepath.Join(custom, "projects", "-p", "a.jsonl")); err != nil || string(got) != "first" {
		t.Errorf("history should land in the configured folder: %q, %v", got, err)
	}
	if _, err := os.Lstat(defaultShared); err == nil {
		t.Errorf("the default %s should be left alone", defaultShared)
	}
}

func TestMigrateRejectsUnknownFlagsAndArguments(t *testing.T) {
	profiles, shared := migrateHome(t)
	twoTranscripts(t, profiles)

	for _, c := range []struct {
		args []string
		msg  string
	}{
		{[]string{"--force"}, "unknown flag --force"},
		{[]string{"-n"}, "unknown flag -n"},
		{[]string{"now"}, "unexpected argument now"},
		{[]string{"--dry-run", "--oops"}, "unknown flag --oops"},
	} {
		code, out, errOut := runMigrate(t, c.args...)
		want := "error: " + c.msg + "\nusage: clalt migrate [--dry-run]\n"
		if code != 2 || out != "" || errOut != want {
			t.Errorf("%v: exit %d, stdout %q, stderr %q, want exit 2 and stderr %q", c.args, code, out, errOut, want)
		}
	}
	if _, err := os.Lstat(shared); err == nil {
		t.Errorf("a rejected command created %s", shared)
	}
}

func TestMigrateExitsOneWhenAFileFails(t *testing.T) {
	profiles, shared := migrateHome(t)
	root := filepath.Join(profiles, "work", "projects", "-p")
	bad := filepath.Join(root, "a-bad.jsonl")
	migrateWrite(t, bad, "private words")
	migrateWrite(t, filepath.Join(root, "b-good.jsonl"), "fine")
	if !lockOut(t, bad) {
		t.Skip("permission bits do not stop this user")
	}

	code, out, errOut := runMigrate(t)
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	for _, want := range []string{"copied 1 file", "1 failed", "failed file: -p/a-bad.jsonl", "permission denied"} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
	if !strings.Contains(errOut, "1 failure") {
		t.Errorf("stderr = %q, want the failure count", errOut)
	}
	if got, err := os.ReadFile(filepath.Join(shared, "-p", "b-good.jsonl")); err != nil || string(got) != "fine" {
		t.Errorf("the good file should still be copied: %q, %v", got, err)
	}
}

func TestMigrateNeverPrintsFileContents(t *testing.T) {
	profiles, shared := migrateHome(t)
	const marker = "TOPSECRET-TRANSCRIPT-WORDS"
	root := filepath.Join(profiles, "work", "projects", "-p")
	migrateWrite(t, filepath.Join(root, "copied.jsonl"), marker+" copied")
	migrateWrite(t, filepath.Join(root, "clash.jsonl"), marker+" mine")
	migrateWrite(t, filepath.Join(root, "bad.jsonl"), marker+" bad")
	migrateWrite(t, filepath.Join(shared, "-p", "clash.jsonl"), marker+" theirs")
	lockOut(t, filepath.Join(root, "bad.jsonl")) // a failure in the mix where it can be made

	for _, args := range [][]string{{}, {"--dry-run"}, {"--json"}, {"--dry-run", "--json"}} {
		_, out, errOut := runMigrate(t, args...)
		if strings.Contains(out, marker) || strings.Contains(errOut, marker) {
			t.Errorf("%v printed file contents:\n%s\n%s", args, out, errOut)
		}
	}
}

func TestUsageListsMigrate(t *testing.T) {
	app, out, _ := newTestApp(t)
	app.usage()
	if !strings.Contains(out.String(), "clalt migrate [--dry-run]") {
		t.Errorf("the help should list migrate:\n%s", out.String())
	}
}
