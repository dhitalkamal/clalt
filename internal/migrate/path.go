package migrate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Inside reports whether path is dir itself or sits below it. it only compares
// the two strings and resolves no links.
func Inside(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && filepath.IsLocal(rel)
}

// ResolveExisting resolves the links in the part of path that exists and keeps
// the rest as written, so a folder that is not created yet can still be
// compared with a resolved root. a name that exists but leads nowhere is an
// error: it is not a folder that is merely missing.
func ResolveExisting(path string) (string, error) {
	rest := ""
	for p := filepath.Clean(path); ; {
		target, err := filepath.EvalSymlinks(p)
		if err == nil {
			return filepath.Join(target, rest), nil
		}
		parent := filepath.Dir(p)
		if !errors.Is(err, fs.ErrNotExist) || parent == p {
			return "", err
		}
		if _, lerr := os.Lstat(p); lerr == nil {
			return "", fmt.Errorf("%s is a link that leads nowhere: %w", p, err)
		}
		rest = filepath.Join(filepath.Base(p), rest)
		p = parent
	}
}

// contain refuses a destination folder that leads out of the shared folder, as
// a link inside it would. it runs before anything is created or written.
func (c *copier) contain(dir string) error {
	resolved, err := ResolveExisting(dir)
	if err != nil {
		return err
	}
	if !Inside(c.destReal, resolved) {
		return fmt.Errorf("folder leads to %s, outside the shared folder", resolved)
	}
	return nil
}
