package profile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Report lists what a Sync did. Linked and Removed are names. Repointed and
// Kept carry the whole step, with the old target and the reason, since those are
// the ones a person has to look at. Kept are real files or folders that sit
// where a link belongs, and entries that could not be read; they were left as
// they are.
type Report struct {
	Linked    []string
	Repointed []Step
	Removed   []string
	Adopted   []Adoption
	Kept      []Step
	Errors    []ItemError
}

// Err joins the item errors, or is nil when there are none.
func (r Report) Err() error {
	errs := make([]error, len(r.Errors))
	for i, e := range r.Errors {
		errs[i] = e
	}
	return errors.Join(errs...)
}

// Complete is false when anything was kept or failed, which means the account
// is not fully linked to the shared home.
func (r Report) Complete() bool { return len(r.Kept) == 0 && len(r.Errors) == 0 }

// ItemError is the failure of one name in a Sync.
type ItemError struct {
	Name string
	Err  error
}

// Error names the entry and gives the cause.
func (e ItemError) Error() string { return e.Name + ": " + e.Err.Error() }

// Unwrap gives the cause, so errors.Is and errors.As see through it.
func (e ItemError) Unwrap() error { return e.Err }

// symlink is a seam for tests: creating one fails on Windows without
// Developer Mode and cannot be provoked portably otherwise.
var symlink = os.Symlink

// rename is the same kind of seam, for the step that moves a new link into place.
var rename = os.Rename

// mkdir is a seam as well, to put something in the place a folder is about to go.
var mkdir = os.Mkdir

// goos is a seam so the windows fallback can be tested anywhere.
var goos = runtime.GOOS

// Sync applies Plan to a profile folder, creating the folder if needed. a failure
// on one name is recorded in the report and does not stop the rest. nothing is
// ever copied in place of a link, so a platform that cannot make symlinks ends
// up with a list of errors and an unlinked profile.
func Sync(profileDir, sharedHome string) (Report, error) {
	var rep Report
	if err := checkDirs(profileDir, sharedHome); err != nil {
		return rep, err
	}
	// taking the lock makes the folder, so it exists for the login that follows
	// even when there is nothing to link
	unlock, err := lockAccount(profileDir)
	if err != nil {
		return rep, err
	}
	defer unlock()
	steps, err := Plan(profileDir, sharedHome)
	if err != nil {
		return rep, err
	}
	// split folders whose old link could not be replaced: nothing goes inside
	// them, because it would land in the shared folder
	blocked := map[string]bool{}
	for _, st := range steps {
		if st.Action == Keep {
			rep.Kept = append(rep.Kept, st)
			continue
		}
		if folder, _, nested := strings.Cut(st.Name, "/"); nested && blocked[folder] {
			continue
		}
		if st.Action == Adopt {
			ad, done, err := adopt(profileDir, st)
			switch {
			case err != nil:
				rep.Errors = append(rep.Errors, ItemError{Name: st.Name, Err: err})
			case done && ad.Name != "":
				rep.Adopted = append(rep.Adopted, ad)
			case done:
				rep.Linked = append(rep.Linked, st.Name)
			}
			continue
		}
		done, err := apply(profileDir, st)
		switch {
		case err != nil:
			if st.Folder {
				blocked[st.Name] = true
			}
			rep.Errors = append(rep.Errors, ItemError{Name: st.Name, Err: err})
		case !done:
			// another sync got there first
		case st.Action == Link:
			rep.Linked = append(rep.Linked, st.Name)
		case st.Action == Repoint:
			rep.Repointed = append(rep.Repointed, st)
		case st.Action == Remove:
			rep.Removed = append(rep.Removed, st.Name)
		}
	}
	return rep, nil
}

// apply does one step. done is false when the tree already looked the way the
// step wanted, which happens when two runs of the same account sync at once.
func apply(profileDir string, st Step) (bool, error) {
	path := filepath.Join(profileDir, filepath.FromSlash(st.Name))
	if err := checkParent(profileDir, st); err != nil {
		return false, err
	}
	switch st.Action {
	case Link:
		if fi, err := os.Lstat(path); err == nil && fi.IsDir() {
			// the plan found this folder empty. os.Remove refuses one with
			// anything in it, so nothing that arrived since can be lost
			if err := os.Remove(path); err != nil {
				return false, fmt.Errorf("the folder in the way is not empty any more, left alone: %w", err)
			}
		}
		err := symlink(st.Source, path)
		if errors.Is(err, fs.ErrExist) && linksTo(path, st.Source) {
			return false, nil
		}
		return err == nil, err
	case Repoint:
		if st.Folder {
			return replaceWithFolder(path)
		}
		return replaceLink(st.Source, path)
	case Remove:
		return removeLink(path)
	}
	return false, fmt.Errorf("cannot apply %v", st.Action)
}

// checkParent guards the entries of a split folder. the folder must be a real
// one: written through a link, a change would land in the shared folder. for a
// new link a missing folder is made.
func checkParent(profileDir string, st Step) error {
	parent, _, nested := strings.Cut(st.Name, "/")
	if !nested {
		return nil
	}
	dir := filepath.Join(profileDir, parent)
	err := realFolder(dir, parent)
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if st.Action != Link {
		return nil
	}
	if err := mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	// something may have taken the name first: look again, never assume
	return realFolder(dir, parent)
}

// realFolder is nil for a real folder, wraps fs.ErrNotExist when there is
// nothing at dir, and says why for anything else. name is how the folder is
// called in the messages.
func realFolder(dir, name string) error {
	fi, err := os.Lstat(dir)
	switch {
	case err != nil:
		return err
	case fi.Mode()&fs.ModeSymlink != 0:
		return fmt.Errorf("%s is a link, nothing is changed through it", name)
	case !fi.IsDir():
		return fmt.Errorf("%s is not a folder", name)
	}
	return nil
}

// replaceWithFolder swaps a link for a real folder. only the link is removed.
func replaceWithFolder(path string) (bool, error) {
	fi, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return false, err
	case fi.Mode()&fs.ModeSymlink == 0:
		if fi.IsDir() {
			return false, nil
		}
		return false, errors.New("not a link any more, left alone")
	default:
		if err := os.Remove(path); err != nil {
			return false, err
		}
	}
	// claude may have made the folder in the meantime, which is just as good
	if err := mkdir(path, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return false, err
	}
	return true, nil
}

// replaceLink aims the link at path at source without a moment where the name
// does not exist: the new link is made under a temporary name and renamed over
// the old one. a name that is not a link any more is left alone, since a rename
// would replace a real file without a word.
func replaceLink(source, path string) (bool, error) {
	fi, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		err := symlink(source, path)
		return err == nil, err
	case err != nil:
		return false, err
	case fi.Mode()&fs.ModeSymlink == 0:
		return false, errors.New("not a link any more, left alone")
	case linksTo(path, source):
		return false, nil
	}
	tmp, err := tempLink(source, path)
	if err != nil {
		return false, err
	}
	if err := rename(tmp, path); err != nil {
		os.Remove(tmp)
		if goos != "windows" {
			return false, err
		}
		// a rename cannot replace a link to a folder there, so the name is absent
		// for a moment. a later sync makes the link again if this one fails
		if err := os.Remove(path); err != nil {
			return false, err
		}
		err := symlink(source, path)
		return err == nil, err
	}
	return true, nil
}

// tempLink makes a link to source next to path under a name of its own.
func tempLink(source, path string) (string, error) {
	dir, base := filepath.Split(path)
	for i := 0; i < 16; i++ {
		tmp := filepath.Join(dir, fmt.Sprintf(".%s.clalt-%d-%d", base, os.Getpid(), i))
		err := symlink(source, tmp)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return tmp, err
	}
	return "", errors.New("no free name for a temporary link")
}

// removeLink deletes a symlink and nothing else: the plan was made earlier, so
// check again that the name is still a link and not a file the user put there.
func removeLink(path string) (bool, error) {
	fi, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if fi.Mode()&fs.ModeSymlink == 0 {
		return false, errors.New("not a link any more, left alone")
	}
	if err := os.Remove(path); err != nil {
		return false, err
	}
	return true, nil
}

func linksTo(path, source string) bool {
	target, err := linkTarget(path)
	return err == nil && pointsAt(target, source)
}
