package cli

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/migrate"
)

func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"plain-name_1.jsonl":     "plain-name_1.jsonl",
		"-Users-x-app/a b.jsonl": "-Users-x-app/a b.jsonl",
		"caf\u00e9":              "caf\u00e9",
		"\u540d\u524d":           "\u540d\u524d",
		"a\nb":                   `a\x0ab`,
		"a\rb":                   `a\x0db`,
		"a\tb":                   `a\x09b`,
		"a\x00b":                 `a\x00b`,
		"a\x7fb":                 `a\x7fb`,
		"a\x1b[31mb":             `a\x1b[31mb`,
		"a\u009bb":               `a\x9bb`,
		"a\u202eb":               `a\u202eb`,
		"a\u200bb":               `a\u200bb`,
		"a\u2028b":               `a\u2028b`,
		"a\U000e0001b":           `a\U000e0001b`,
		"a\xffb":                 `a\xffb`,
	} {
		if got := clean(in); got != want {
			t.Errorf("clean(%q) = %q, want %q", in, got, want)
		}
	}
}

// onlyOurNewlines fails when the report holds a control character it did not
// print itself: only the line breaks between its own lines are allowed.
func onlyOurNewlines(t *testing.T, label, text string) {
	t.Helper()
	for _, r := range text {
		if r != '\n' && (unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.In(r, unicode.Zl, unicode.Zp)) {
			t.Errorf("%s holds the character %U that was not escaped:\n%q", label, r, text)
			return
		}
	}
}

func hostileScenario(t *testing.T) (conflict, skipped string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("windows cannot create these names")
	}
	profiles, shared := migrateHome(t)
	conflict = "clash\ntotal: copied 9999 files\n.jsonl"
	skipped = "link\x1b[2J\u202e.jsonl"
	evil := filepath.Join(profiles, "bad\nprofile\x1b[31m", "projects", "-p")
	migrateWrite(t, filepath.Join(evil, conflict), "mine")
	migrateWrite(t, filepath.Join(shared, "-p", conflict), "theirs")
	mustSymlink(t, "target\x07\nforged: yes", filepath.Join(evil, skipped))
	return conflict, skipped
}

func TestMigrateTextReportEscapesNames(t *testing.T) {
	conflict, skipped := hostileScenario(t)
	_ = skipped

	for _, args := range [][]string{{}, {"--dry-run"}} {
		code, out, errOut := runMigrate(t, args...)
		if code != 0 || errOut != "" {
			t.Fatalf("%v: exit %d, stderr %q", args, code, errOut)
		}
		onlyOurNewlines(t, "the text report", out)
		totals := 0
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "total:") {
				totals++
			}
			if strings.HasPrefix(line, "forged:") || strings.HasPrefix(line, ".jsonl") {
				t.Errorf("a name forged the line %q", line)
			}
		}
		if totals != 1 {
			t.Errorf("%d lines start with total:, want 1:\n%s", totals, out)
		}
		if !strings.Contains(out, clean(conflict)) {
			t.Errorf("the conflict should still be named, escaped as %q:\n%s", clean(conflict), out)
		}
	}
}

func TestMigrateJSONKeepsTheRawNames(t *testing.T) {
	conflict, skipped := hostileScenario(t)

	code, out, errOut := runMigrate(t, "--json")
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	var rep migrate.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	p := rep.Profiles[0]
	if p.Name != "bad\nprofile\x1b[31m" || p.Conflicts[0].Path != "-p/"+conflict || p.SkippedEntries[0].Path != "-p/"+skipped ||
		p.SkippedEntries[0].Target != "target\x07\nforged: yes" {
		t.Errorf("json should carry the names as they are: %+v", p)
	}
}

func TestMigrateErrorsEscapeWhatTheyPrint(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows cannot create these names")
	}
	profiles, _ := migrateHome(t)
	twoTranscripts(t, profiles)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(config.ProfileDir("work"), "x\x1b[2J\nforged"))

	code, out, errOut := runMigrate(t)
	if code != 1 || out != "" {
		t.Fatalf("exit %d, stdout %q", code, out)
	}
	onlyOurNewlines(t, "the refusal", errOut)

	_, _, usage := runMigrate(t, "--bad\x1b[2J")
	onlyOurNewlines(t, "the usage error", usage)
}

// the next four pin the clean calls the scenarios above do not reach: the
// folder names in the two headers, the no-profiles line, the error that
// migrate.Run returns, and a failed file's path and message plus a link target.

func TestHeaderNamesAreCleaned(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows cannot create these names")
	}
	profiles, _ := migrateHome(t)
	twoTranscripts(t, profiles)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(t.TempDir(), "my\x1b[2Jclaude"))
	for _, args := range [][]string{{}, {"--dry-run"}} {
		_, out, _ := runMigrate(t, args...)
		onlyOurNewlines(t, "the header", out)
	}
}

func TestNoProfilesHeaderIsCleaned(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows cannot create these names")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg\x1b[2J"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	_, out, _ := runMigrate(t)
	onlyOurNewlines(t, "the no-profiles line", out)
}

func TestRunErrorsAreCleaned(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows cannot create these names")
	}
	profiles, shared := migrateHome(t)
	twoTranscripts(t, profiles)
	mustSymlink(t, "/nowhere\x1b[2J/x", shared)
	code, out, errOut := runMigrate(t)
	if code != 1 || out != "" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	onlyOurNewlines(t, "the run error", errOut)
}

func TestFailureAndLinkNamesAreCleaned(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows cannot create these names")
	}
	profiles, _ := migrateHome(t)
	bad := filepath.Join(profiles, "work", "projects", "-p", "bad\x1b[2J.jsonl")
	migrateWrite(t, bad, "private words")
	if !lockOut(t, bad) {
		t.Skip("permission bits do not stop this user")
	}
	elsewhere := filepath.Join(t.TempDir(), "else\x1b[2J")
	migrateWrite(t, filepath.Join(elsewhere, "x.jsonl"), "x")
	mustSymlink(t, elsewhere, filepath.Join(profiles, "stray", "projects"))

	_, out, _ := runMigrate(t)
	onlyOurNewlines(t, "the report", out)
}
