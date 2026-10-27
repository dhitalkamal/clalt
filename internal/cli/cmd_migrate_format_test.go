package cli

import (
	"fmt"
	"path/filepath"
	"testing"
)

// formatScenario builds one profile of every kind and returns the shared
// folder, the real place the foreign link leads to, and the link inside beta as
// it was written:
//
//	alpha  projects links to the shared folder
//	beta   a real folder: two new files, one identical, one conflict, one link
//	delta  projects links to a folder elsewhere
//	gamma  no projects at all
func formatScenario(t *testing.T) (shared, elsewhere, linked string) {
	t.Helper()
	profiles, shared := migrateHome(t)
	elsewhere = filepath.Join(t.TempDir(), "elsewhere")
	linked = filepath.Join(elsewhere, "x.jsonl") // as written in the link, unresolved
	migrateWrite(t, filepath.Join(elsewhere, "x.jsonl"), "not ours")
	migrateWrite(t, filepath.Join(shared, "-p", "same.jsonl"), "same")
	migrateWrite(t, filepath.Join(shared, "-p", "clash.jsonl"), "theirs")

	beta := filepath.Join(profiles, "beta", "projects", "-p")
	migrateWrite(t, filepath.Join(beta, "a.jsonl"), "first")
	migrateWrite(t, filepath.Join(beta, "b.jsonl"), "second!")
	migrateWrite(t, filepath.Join(beta, "same.jsonl"), "same")
	migrateWrite(t, filepath.Join(beta, "clash.jsonl"), "mine")
	migrateWrite(t, filepath.Join(profiles, "gamma", "settings.json"), "{}")
	for path, target := range map[string]string{
		filepath.Join(beta, "ln.jsonl"):              filepath.Join(elsewhere, "x.jsonl"),
		filepath.Join(profiles, "alpha", "projects"): shared,
		filepath.Join(profiles, "delta", "projects"): elsewhere,
	} {
		mustSymlink(t, target, path)
	}
	target, err := filepath.EvalSymlinks(elsewhere)
	if err != nil {
		t.Fatal(err)
	}
	return shared, target, linked
}

func TestMigrateOutputFormat(t *testing.T) {
	shared, elsewhere, linked := formatScenario(t)

	_, out, _ := runMigrate(t)
	want := fmt.Sprintf(`migrating profile history into %s

  alpha  already shared
  beta   copied 2 files (12 B), 1 already there, 1 conflict, 1 skipped (links or special files)
         conflict: -p/clash.jsonl (the shared folder holds a different file)
         skipped: -p/ln.jsonl (link to %s)
  delta  projects is a link to %s, not copied
  gamma  nothing to copy

total: copied 2 files (12 B), 1 already there, 1 conflict, 1 skipped (links or special files), 1 link not followed
the profile folders were left as they were: this only copies.
a conflict means the shared folder already holds a different file at that path. the profile's version is only in the profile folder, so keep it.
links are never followed, so what they lead to was not copied.
`, shared, linked, elsewhere)
	if out != want {
		t.Errorf("output differs\n--- got\n%s--- want\n%s", out, want)
	}
}

func TestMigrateDryRunOutputFormat(t *testing.T) {
	shared, elsewhere, linked := formatScenario(t)

	_, out, _ := runMigrate(t, "--dry-run")
	want := fmt.Sprintf(`dry run, nothing is written. profile history would go into %s

  alpha  already shared
  beta   would copy 2 files (12 B), 1 already there, 1 conflict, 1 skipped (links or special files)
         conflict: -p/clash.jsonl (the shared folder holds a different file)
         skipped: -p/ln.jsonl (link to %s)
  delta  projects is a link to %s, not copied
  gamma  nothing to copy

total: would copy 2 files (12 B), 1 already there, 1 conflict, 1 skipped (links or special files), 1 link not followed
nothing was written. a dry run does not check free space or write permission.
run 'clalt migrate' to copy for real.
`, shared, linked, elsewhere)
	if out != want {
		t.Errorf("output differs\n--- got\n%s--- want\n%s", out, want)
	}
}

func TestHumanBytes(t *testing.T) {
	for n, want := range map[int64]string{
		0:        "0 B",
		1:        "1 B",
		1023:     "1023 B",
		1024:     "1.0 KB",
		1536:     "1.5 KB",
		41 << 20: "41.0 MB",
		5 << 30:  "5.0 GB",
		3 << 40:  "3.0 TB",
	} {
		if got := humanBytes(n); got != want {
			t.Errorf("humanBytes(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestCount(t *testing.T) {
	for _, c := range []struct {
		n    int
		noun string
		want string
	}{
		{0, "file", "0 files"},
		{1, "file", "1 file"},
		{2, "conflict", "2 conflicts"},
		{1, "failure", "1 failure"},
	} {
		if got := count(c.n, c.noun); got != c.want {
			t.Errorf("count(%d, %q) = %q, want %q", c.n, c.noun, got, c.want)
		}
	}
}
