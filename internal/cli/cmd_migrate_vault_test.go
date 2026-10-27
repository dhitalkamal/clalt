package cli

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// captureRun runs the real entry point with stdout and stderr captured, so what
// it opens and creates is the real thing, not a test app.
func captureRun(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()

	done := make(chan [2]string)
	go func() {
		o, _ := io.ReadAll(outR)
		e, _ := io.ReadAll(errR)
		done <- [2]string{string(o), string(e)}
	}()
	code = Run(args)
	outW.Close()
	errW.Close()
	got := <-done
	return code, got[0], got[1]
}

// migrate only copies files. opening the vault would create vault.key on a
// machine that has never run another command.
func TestRunMigrateLeavesAFreshMachineAlone(t *testing.T) {
	migrateHome(t)

	code, out, errOut := captureRun(t, "migrate", "--dry-run")
	if code != 0 || errOut != "" || !strings.Contains(out, "profile folders found") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	if _, err := os.Lstat(config.ClaltDir()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a dry run created %s: %v", config.ClaltDir(), err)
	}
}

func TestRunMigrateDoesNotOpenTheVault(t *testing.T) {
	for name, args := range map[string][]string{
		"dry run":      {"migrate", "--dry-run"},
		"real run":     {"migrate"},
		"json":         {"migrate", "--json"},
		"json first":   {"--json", "migrate", "--dry-run"},
		"dry run json": {"migrate", "--dry-run", "--json"},
	} {
		t.Run(name, func(t *testing.T) {
			profiles, _ := migrateHome(t)
			twoTranscripts(t, profiles)
			before := treeNames(t, config.ClaltDir())

			code, out, errOut := captureRun(t, args...)
			if code != 0 || errOut != "" || out == "" {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
			}
			after := treeNames(t, config.ClaltDir())
			if strings.Join(after, "\n") != strings.Join(before, "\n") {
				t.Errorf("migrate changed the clalt folder:\n  was %v\n  now %v", before, after)
			}
			for _, vault := range []string{config.VaultDir(), config.VaultKeyPath()} {
				if _, err := os.Lstat(vault); !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("migrate created %s: %v", filepath.Base(vault), err)
				}
			}
		})
	}
}
