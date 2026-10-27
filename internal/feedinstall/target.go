package feedinstall

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ownedByUser says whether the file belongs to the current user. a variable so
// tests can pretend otherwise.
var ownedByUser = fileOwnedByUser

// Target is the file an install or uninstall really edits: settings.json itself,
// or what it links to.
type Target struct {
	Path    string // the file to read and write
	Link    bool   // settings.json is a symlink
	GitRoot string // work tree that holds Path, when it is in one
}

// ResolveTarget works out the real file behind settings.json and refuses one
// clalt must not edit: a dangling link, a loop, anything that is not a regular
// file, a file of someone else, or one inside clalt's own folders.
func ResolveTarget(o Options) (Target, error) { return resolve(o, true) }

// resolve follows the link. lenient (strict false) skips the owner and folder
// checks, which only matter for a write: status just reads.
func resolve(o Options, strict bool) (Target, error) {
	fi, err := os.Lstat(o.Settings)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Target{Path: o.Settings}, nil
		}
		return Target{}, err
	}
	if fi.Mode()&fs.ModeSymlink == 0 {
		return Target{Path: o.Settings}, nil
	}
	t := Target{Link: true}
	p, err := filepath.EvalSymlinks(o.Settings)
	switch {
	case err == nil:
	case errors.Is(err, fs.ErrNotExist):
		return t, fmt.Errorf("%s is a link to %s, which does not exist: nothing was changed", o.Settings, readLink(o.Settings))
	case strings.Contains(err.Error(), "too many links") || strings.Contains(err.Error(), "too many levels"):
		return t, fmt.Errorf("%s is part of a link loop: nothing was changed", o.Settings)
	default:
		return t, fmt.Errorf("follow the link %s: %w", o.Settings, err)
	}
	t.Path = p
	tfi, err := os.Lstat(p)
	if err != nil {
		return t, err
	}
	if !tfi.Mode().IsRegular() {
		return t, fmt.Errorf("%s (the target of %s) is not a regular file: nothing was changed", p, o.Settings)
	}
	if !strict {
		return t, nil
	}
	if !ownedByUser(p) {
		return t, fmt.Errorf("%s (the target of %s) is not yours: clalt will not edit it", p, o.Settings)
	}
	for _, dir := range o.Protected {
		if inside(p, dir) {
			return t, fmt.Errorf("%s is inside clalt's own folder %s: clalt will not edit it", p, dir)
		}
	}
	t.GitRoot = gitRoot(p)
	return t, nil
}

func readLink(p string) string {
	if l, err := os.Readlink(p); err == nil {
		return l
	}
	return "?"
}

// inside says whether p is dir or below it, comparing real paths when both exist.
func inside(p, dir string) bool {
	if d, err := filepath.EvalSymlinks(dir); err == nil {
		dir = d
	}
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// gitRoot walks up from the file looking for a .git entry.
func gitRoot(p string) string {
	for dir := filepath.Dir(p); ; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		if dir == filepath.Dir(dir) {
			return ""
		}
	}
}
