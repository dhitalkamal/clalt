package profile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type entryKind int

const (
	skipEntry entryKind = iota
	linkEntry
	splitEntry
)

// rules say what happens to an entry of a shared folder. couldLink is asked
// when the entry cannot be read, so its kind is unknown: would it be linked
// for some kind?
type rules struct {
	classify  func(name string, isDir bool) entryKind
	couldLink func(name string) bool
}

// homeRules are the rules for the shared home itself.
var homeRules = rules{
	classify: func(name string, isDir bool) entryKind {
		switch {
		case isDir && splitDirs[name] != nil:
			return splitEntry
		case shareable(name, isDir):
			return linkEntry
		}
		return skipEntry
	},
	couldLink: func(name string) bool {
		return splitDirs[name] != nil || shareable(name, true) || shareable(name, false)
	},
}

// insideRules are the rules for the entries of a split folder: all of them are
// linked, files and folders alike, except the names the account keeps and the
// ones that are never linked. no shared folder should hold the latter, which is
// why they are checked here too instead of trusted to be absent.
func insideRules(split string) rules {
	left := splitDirs[split]
	skip := func(name string) bool { return left[name] || neverLink[name] }
	return rules{
		classify: func(name string, _ bool) entryKind {
			if skip(name) {
				return skipEntry
			}
			return linkEntry
		},
		couldLink: func(name string) bool { return !skip(name) },
	}
}

// sharedView is what a shared folder holds that matters for linking.
type sharedView struct {
	link       map[string]string // name -> its path in the shared folder
	split      map[string]string // same, for the folders that are split
	unreadable map[string]error  // names that could be linked but cannot be examined
}

// known lists every name the shared folder still has a say about, so a link in
// the account for one of them is never taken for a stale one.
func (v sharedView) known() map[string]bool {
	out := map[string]bool{}
	for _, m := range []map[string]string{v.link, v.split} {
		for name := range m {
			out[name] = true
		}
	}
	for name := range v.unreadable {
		out[name] = true
	}
	return out
}

// addMade adds the folders that the caller is about to make, as readShared would
// have sorted them once they were there. a name that is there in any form, a
// link to nothing included, is not made up, and neither is one that cannot be
// looked at.
func (v sharedView) addMade(dir string, names []string) {
	for _, name := range names {
		path := filepath.Join(dir, name)
		if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
			v.link[name] = path
		}
	}
}

// readShared sorts the entries of a shared folder by the rules. a symlink is
// classified by what it points to. only a name that is absent from the listing
// is gone. one that is listed but cannot be examined (a link whose target is
// missing, a link loop, permissions) has an unknown kind, so it is recorded as
// unreadable when it could be linked. an error reading the folder itself is
// returned as it is.
func readShared(dir string, r rules) (sharedView, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return sharedView{}, err
	}
	v := sharedView{link: map[string]string{}, split: map[string]string{}, unreadable: map[string]error{}}
	for _, e := range entries {
		name := e.Name()
		src := filepath.Join(dir, name)
		fi, err := os.Stat(src)
		switch {
		case err != nil:
			if r.couldLink(name) {
				v.unreadable[name] = err
			}
			continue
		case !(fi.IsDir() || fi.Mode().IsRegular()):
			continue
		}
		switch r.classify(name, fi.IsDir()) {
		case linkEntry:
			v.link[name] = src
		case splitEntry:
			v.split[name] = src
		}
	}
	return v, nil
}

// reason is the part of a file system error after the path.
func reason(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}
