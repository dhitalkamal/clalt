package claudecli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// checkFolder refuses a folder claude must not be pointed at. claude makes the
// folder it is given and writes .claude.json, a lock dir and backups/ into it, so
// an empty or relative path would end up wherever clalt happens to be, a missing
// folder would be created by claude behind clalt's back, and the shared home
// would have its own login replaced or signed out.
func checkFolder(dir string) error { return checkFolderFor(dir, false) }

// checkFolderFor is checkFolder, with the shared home allowed for a call that
// only reads.
func checkFolderFor(dir string, allowShared bool) error {
	switch {
	case dir == "":
		return errors.New("no account folder given")
	case !filepath.IsAbs(dir):
		return fmt.Errorf("the account folder must be an absolute path, got %q", dir)
	}
	fi, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("the account folder %s does not exist, create it first", dir)
	case err != nil:
		return err
	case !fi.IsDir():
		return fmt.Errorf("%s is not a folder", dir)
	}
	if shared := config.SharedClaudeDir(); !allowShared && shared != "" && fsutil.SamePath(dir, shared) {
		return fmt.Errorf("%s is the shared claude home, not an account folder", dir)
	}
	return nil
}
