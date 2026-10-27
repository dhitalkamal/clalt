package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// aliasOf returns a link in a fresh folder that leads to target, so one place
// can be spelled two ways even where the temp folder has no links of its own,
// as on linux CI.
func aliasOf(t *testing.T, target string) string {
	t.Helper()
	link := filepath.Join(t.TempDir(), "alias")
	mustSymlink(t, target, link)
	return link
}

// the folder is not made yet, so only the part that exists can be resolved,
// and what is missing has to be kept rather than dropped.
func TestMigrateRefusesAFolderNotMadeYetBelowClaltThroughAnAlias(t *testing.T) {
	profiles, shared := migrateHome(t)
	twoTranscripts(t, profiles)
	alias := aliasOf(t, config.ClaltDir())
	dir := filepath.Join(alias, "profiles", "newaccount")
	t.Setenv("CLAUDE_CONFIG_DIR", dir)

	code, out, errOut := runMigrate(t)
	wantRefusal(t, code, out, errOut, dir)
	if _, err := os.Lstat(shared); err == nil {
		t.Errorf("a refused run created %s", shared)
	}
}

// on a fresh machine clalt's own folder is not there yet either, and
// XDG_CONFIG_HOME may itself be an alias while the account folder is spelled
// by its real name.
func TestMigrateRefusesWhenClaltIsNotThereYetAndXDGIsAnAlias(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	realConfig := filepath.Join(home, ".config")
	if err := os.MkdirAll(realConfig, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", aliasOf(t, realConfig))
	dir := filepath.Join(realConfig, "clalt", "profiles", "work")
	t.Setenv("CLAUDE_CONFIG_DIR", dir)

	code, out, errOut := runMigrate(t)
	wantRefusal(t, code, out, errOut, dir)
}

func TestMigrateTreatsADestinationSpelledThroughAnAliasAsTheSharedFolder(t *testing.T) {
	profiles, _ := migrateHome(t)
	custom := filepath.Join(t.TempDir(), "my-claude")
	migrateWrite(t, filepath.Join(custom, "projects", "-p", "there.jsonl"), "there")
	migrateWrite(t, filepath.Join(profiles, "beta", "projects", "-p", "new.jsonl"), "fresh")
	mustSymlink(t, filepath.Join(custom, "projects"), filepath.Join(profiles, "work", "projects"))
	t.Setenv("CLAUDE_CONFIG_DIR", aliasOf(t, custom))

	code, out, errOut := runMigrate(t)
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	mustHave(t, out, "work  already shared", "beta  copied 1 file (5 B)")
	if got, err := os.ReadFile(filepath.Join(custom, "projects", "-p", "new.jsonl")); err != nil || string(got) != "fresh" {
		t.Errorf("the copy should land in the real folder: %q, %v", got, err)
	}
}

func TestMigrateMakesADestinationThatIsNotThereYetThroughAnAlias(t *testing.T) {
	profiles, _ := migrateHome(t)
	twoTranscripts(t, profiles)
	parent := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(aliasOf(t, parent), "new-claude"))

	code, _, errOut := runMigrate(t)
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if got, err := os.ReadFile(filepath.Join(parent, "new-claude", "projects", "-p", "a.jsonl")); err != nil || string(got) != "first" {
		t.Errorf("the copy should land under the real parent: %q, %v", got, err)
	}
}

// a relative path that lands inside clalt's folder is still an account folder,
// and deserves the same explanation as an absolute one.
func TestMigrateRefusesARelativeFolderBelowClalt(t *testing.T) {
	profiles, _ := migrateHome(t)
	twoTranscripts(t, profiles)
	t.Chdir(config.ClaltDir())
	dir := filepath.Join("profiles", "work")
	t.Setenv("CLAUDE_CONFIG_DIR", dir)

	code, out, errOut := runMigrate(t)
	wantRefusal(t, code, out, errOut, dir)
}
