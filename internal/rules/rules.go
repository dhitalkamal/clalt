// Package rules decides whether an account may serve a folder. an account has a
// list of folder rules: each one matches that folder and everything under it,
// and an account with no rules may serve any folder.
package rules

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// Why a rule can match nothing, see Problems.
const (
	WhyNotAbsolute = "is not an absolute path"
	WhyGone        = "does not exist"
)

// Problem is a rule that can match nothing, and why.
type Problem struct{ Folder, Why string }

// Normalize makes a folder argument into what is stored: absolute and clean, and
// otherwise as it was written. a relative path is taken against the working
// directory. a link is not resolved here: where a rule leads is worked out when
// it is matched, so a link that moves later is followed.
func Normalize(path string) (string, error) {
	switch {
	case strings.TrimSpace(path) == "":
		return "", errors.New("the folder is empty")
	case strings.ContainsRune(path, 0):
		return "", errors.New("the folder contains a NUL byte")
	case path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`):
		// a quoted ~ reaches clalt as it was typed, and would become a folder
		// called ~ under the working directory
		return "", fmt.Errorf("the shell did not expand ~ in %q: give the full path", path)
	}
	return filepath.Abs(path)
}

// Allows says whether an account with these rules may serve cwd. no rules means
// any folder, even when cwd could not be read. a rule that is not an absolute
// path never matches, so an account whose rules are all like that serves nothing.
// the file system decides what is the same folder, so links are followed and a
// file system that ignores case ignores it here too.
func Allows(folders []string, cwd string) bool {
	if len(folders) == 0 {
		return true
	}
	if cwd == "" {
		return false
	}
	for _, f := range folders {
		// both the spelling and where it leads must be inside: a cwd kept as typed
		// after a cd through a link can lead anywhere
		if filepath.IsAbs(f) && fsutil.Within(cwd, f) && fsutil.Within(fsutil.ResolvePath(cwd), f) {
			return true
		}
	}
	return false
}

// Same says whether two stored folders lead to the same place, whatever the
// spelling, a link or the case of a name.
func Same(a, b string) bool { return fsutil.SamePath(a, b) }

// Problems lists the rules that can match nothing: a path that is not absolute,
// and a folder that is not there. a folder that cannot be looked at, for lack of
// permission, may well be there and is not listed.
func Problems(folders []string) []Problem {
	var out []Problem
	for _, f := range folders {
		switch _, err := os.Stat(f); {
		case !filepath.IsAbs(f):
			out = append(out, Problem{Folder: f, Why: WhyNotAbsolute})
		case errors.Is(err, fs.ErrNotExist):
			out = append(out, Problem{Folder: f, Why: WhyGone})
		}
	}
	return out
}
