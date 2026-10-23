package profile

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// ErrNoSharedHome means the shared claude home does not exist. there is then
// nothing to link, and links into it cannot be told from stale ones: the home
// may only be unmounted, so nothing is removed.
var ErrNoSharedHome = errors.New("no shared claude home")

// Action says what a Step does to its name in the profile folder.
type Action int

// Link, Repoint, Remove and Keep are the actions of a Step. Keep changes
// nothing: it reports a name that was left as it is, with the reason in Why.
const (
	Link Action = iota + 1
	Repoint
	Remove
	Keep
)

// String gives the lowercase name of the action, for messages.
func (a Action) String() string {
	switch a {
	case Link:
		return "link"
	case Repoint:
		return "repoint"
	case Remove:
		return "remove"
	case Keep:
		return "keep"
	}
	return "unknown"
}

// Step is one thing Sync would do, or report, for a name in a profile folder.
// Name is relative to the profile folder and slash separated, so an entry
// inside a split folder reads "skills/review". Source is the shared home entry
// a link aims at (empty for Remove), Old the target of the link being replaced
// or removed, and Why a short reason that doctor can print as is. Folder marks a
// Repoint that does not make a link: it replaces the link with a real folder.
type Step struct {
	Name   string
	Action Action
	Source string
	Old    string
	Why    string
	Folder bool
}

// Plan compares a profile folder with the shared home and lists what would
// bring it in line, sorted by name. it never writes, so doctor can use it as the
// health check. names that are already right are left out. a split folder
// (skills, plugins) comes before its entries.
func Plan(profileDir, sharedHome string) ([]Step, error) {
	return plan(profileDir, sharedHome, nil)
}

func plan(profileDir, sharedHome string, made []string) ([]Step, error) {
	if err := checkDirs(profileDir, sharedHome); err != nil {
		return nil, err
	}
	view, err := readShared(sharedHome, homeRules)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNoSharedHome, sharedHome)
	}
	if err != nil {
		return nil, err
	}
	view.addMade(sharedHome, made)
	steps, err := planFolder(profileDir, sharedHome, "", view)
	if err != nil {
		return nil, err
	}
	for name := range splitDirs {
		var more []Step
		var err error
		if src, ok := view.split[name]; ok {
			more, err = planSplit(profileDir, name, src)
		} else {
			more, err = planLostSplit(profileDir, name, filepath.Join(sharedHome, name), view)
		}
		if err != nil {
			return nil, err
		}
		steps = append(steps, more...)
	}
	sort.Slice(steps, func(i, j int) bool { return steps[i].Name < steps[j].Name })
	return steps, nil
}

// PlanWithFolders is Plan for a shared home that is about to get a folder for
// each of the names, which the caller makes before it syncs: `clalt run` does
// that for a projects folder that is missing. the account is judged against the
// home as it will be, so that doctor can say what run will do. a name that the
// home has in any form is left as it is, and a name that is not linked whole
// (skills and plugins are split, others stay per account) is an error, since
// there is no plain folder to make up for it. it writes nothing.
func PlanWithFolders(profileDir, sharedHome string, folders ...string) ([]Step, error) {
	for _, name := range folders {
		if homeRules.classify(name, true) != linkEntry {
			return nil, fmt.Errorf("%s is not a folder that is linked whole", name)
		}
	}
	return plan(profileDir, sharedHome, folders)
}

// checkDirs rejects what would turn into links aimed at the wrong place: an
// empty path stands for the working directory, a relative one is read from
// wherever the caller happens to be, and a profile folder that is the shared
// home would link entries into themselves.
func checkDirs(profileDir, sharedHome string) error {
	for _, d := range []struct{ what, path string }{{"profile folder", profileDir}, {"shared claude home", sharedHome}} {
		switch {
		case d.path == "":
			return fmt.Errorf("no %s given", d.what)
		case !filepath.IsAbs(d.path):
			return fmt.Errorf("the %s must be an absolute path, got %q", d.what, d.path)
		}
	}
	if fsutil.SamePath(profileDir, sharedHome) {
		return errors.New("the profile folder is the shared claude home")
	}
	return nil
}
