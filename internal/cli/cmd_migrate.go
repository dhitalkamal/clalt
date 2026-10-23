package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/migrate"
)

const migrateUsage = "clalt migrate [--dry-run]"

func (a *App) cmdMigrate(args []string) error {
	dryRun, err := parseMigrateArgs(args)
	if err != nil {
		return err
	}
	dest, err := migrateDest()
	if err != nil {
		return err
	}
	rep, err := migrate.Run(config.ProfilesDir(), dest, migrate.Options{DryRun: dryRun})
	if err != nil {
		return cleanErr(err)
	}
	if a.JSON {
		err = a.emitJSON(rep)
	} else {
		_, err = io.WriteString(a.out(), renderMigrate(rep))
	}
	if err != nil {
		return err
	}
	if n := rep.Total.Failed; n > 0 {
		if rep.DryRun {
			return fmt.Errorf("the dry run found %s, see the failed list", count(n, "failure"))
		}
		return fmt.Errorf("%s while copying, see the failed list", count(n, "failure"))
	}
	return nil
}

// parseMigrateArgs takes --dry-run and nothing else: this command writes
// files, so a mistyped flag is an error rather than something to ignore.
func parseMigrateArgs(args []string) (dryRun bool, err error) {
	for _, arg := range args {
		switch {
		case arg == "--dry-run":
			dryRun = true
		case strings.HasPrefix(arg, "-"):
			return false, usageError{"unknown flag " + clean(arg), migrateUsage}
		default:
			return false, usageError{"unexpected argument " + clean(arg), migrateUsage}
		}
	}
	return dryRun, nil
}

// migrateDest is the shared projects folder. inside a `clalt run` session
// CLAUDE_CONFIG_DIR is an account folder, and the usual lookup would then aim
// the copy at that account's own projects instead of the shared history.
func migrateDest() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" && underClaltDir(dir) {
		return "", fmt.Errorf("CLAUDE_CONFIG_DIR points at an account folder (%s), not at the shared history. "+
			"run 'clalt migrate' from a normal shell, not inside a 'clalt run' session", clean(dir))
	}
	dest := config.ClaudeProjectsDir()
	if dest == "" {
		return "", errors.New(noHomeKnown)
	}
	return dest, nil
}

// underClaltDir is true for clalt's own folder and anything below it, however
// the path is spelled and whether or not the folder exists yet.
func underClaltDir(dir string) bool {
	clalt := config.ClaltDir()
	return migrate.Inside(clalt, dir) || migrate.Inside(resolved(clalt), resolved(dir))
}

// resolved follows the links in the part of p that exists and keeps the rest as
// written. a path that cannot be resolved is left as it is.
func resolved(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if r, err := migrate.ResolveExisting(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}
