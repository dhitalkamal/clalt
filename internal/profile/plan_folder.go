package profile

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// missingTarget is why a Keep names a shared entry that is a link to nothing.
const missingTarget = "the shared entry is a link to something that is missing"

// planFolder compares one folder of the account with its shared counterpart.
// dir is "" when the folder does not exist yet, and prefix is the folder's name
// relative to the profile ("" or "skills/"), put in front of every step name.
func planFolder(dir, sharedDir, prefix string, v sharedView) ([]Step, error) {
	var steps []Step
	for name, src := range v.link {
		st, needed, err := planEntry(dir, prefix, name, src)
		if err != nil {
			return nil, err
		}
		if needed {
			steps = append(steps, st)
		}
	}
	// an entry that cannot be examined is neither linked nor treated as gone
	for name, cause := range v.unreadable {
		src := filepath.Join(sharedDir, name)
		why := "cannot read it in the shared home: " + reason(cause)
		if errors.Is(cause, fs.ErrNotExist) {
			// a link to nothing, say into an unmounted volume. it is only worth a
			// word where the account has a link to it
			if dir == "" || !linksTo(filepath.Join(dir, name), src) {
				continue
			}
			why = missingTarget
		}
		steps = append(steps, Step{Name: prefix + name, Action: Keep, Source: src, Why: why})
	}
	steps, err := adoptMissing(steps, dir, sharedDir, prefix, v)
	if err != nil {
		return nil, err
	}
	stale, err := staleLinks(dir, sharedDir, prefix, v.known())
	if err != nil {
		return nil, err
	}
	return append(steps, stale...), nil
}

// planSplit plans a folder that is split: the account gets a real folder, and
// the entries of the shared one are linked into it. an old link to the whole
// shared folder is replaced first. that is lossless, since only the link goes,
// and it has to happen before anything is linked inside: a link written through
// it would land in the shared folder itself.
func planSplit(profileDir, name, src string) ([]Step, error) {
	inside, err := readShared(src, insideRules(name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return []Step{{Name: name, Action: Keep, Source: src, Why: "cannot read the shared folder: " + reason(err)}}, nil
	}
	path := filepath.Join(profileDir, name)
	dir := path
	var steps []Step
	fi, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		dir = ""
	case err != nil:
		return nil, err
	case fi.Mode()&fs.ModeSymlink != 0:
		target, err := linkTarget(path)
		if err != nil {
			return nil, err
		}
		steps = append(steps, Step{
			Name: name, Action: Repoint, Source: src, Old: target, Folder: true,
			Why: "a link to the whole folder is replaced by a real folder with links inside",
		})
		// the folder is made afresh, so what the old link leads to does not count
		dir = ""
	case !fi.IsDir():
		return []Step{{Name: name, Action: Keep, Source: src, Why: "a real file is in the way"}}, nil
	}
	more, err := planFolder(dir, src, name+"/", inside)
	if err != nil {
		return nil, err
	}
	return append(steps, more...), nil
}

// planLostSplit plans a split folder that the shared home has no readable folder
// for. when the name is absent from the shared home, the links the account made
// into it are stale like any other and only they go. when it is there but cannot
// be reached (a link into an unmounted volume) nothing goes, and the account's
// links into it are reported once. a link or a file where the account's folder
// belongs is planFolder's business.
func planLostSplit(profileDir, name, src string, v sharedView) ([]Step, error) {
	dir := filepath.Join(profileDir, name)
	if fi, err := os.Lstat(dir); err != nil || !fi.IsDir() {
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		return nil, nil
	}
	if cause, listed := v.unreadable[name]; listed && errors.Is(cause, fs.ErrNotExist) {
		links, err := staleLinks(dir, src, name+"/", nil)
		if err != nil || len(links) == 0 {
			return nil, err
		}
		return []Step{{Name: name, Action: Keep, Source: src, Why: missingTarget}}, nil
	}
	if _, err := os.Lstat(src); !errors.Is(err, fs.ErrNotExist) {
		// there but unreachable (planFolder reports it) or not a folder: nothing
		// says the links are stale
		return nil, nil
	}
	return staleLinks(dir, src, name+"/", nil)
}

// planEntry decides what an entry that should be linked needs; needed is false
// when the link is already right.
func planEntry(dir, prefix, name, source string) (st Step, needed bool, err error) {
	link := Step{Name: prefix + name, Action: Link, Source: source, Why: "not linked yet"}
	if dir == "" {
		return link, true, nil
	}
	path := filepath.Join(dir, name)
	fi, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return link, true, nil
	}
	if err != nil {
		return Step{}, false, err
	}
	if fi.Mode()&fs.ModeSymlink == 0 {
		if st, ok := adoptStep(dir, prefix, name, source, fi); ok {
			return st, true, nil
		}
		return inTheWay(prefix+name, source, path, fi), true, nil
	}
	target, err := linkTarget(path)
	if err != nil {
		return Step{}, false, err
	}
	if pointsAt(target, source) {
		return Step{}, false, nil
	}
	return Step{Name: prefix + name, Action: Repoint, Source: source, Old: target, Why: "the link aims somewhere else"}, true, nil
}

// inTheWay decides what to do about a real file or folder where a link
// belongs. a file is kept. so is a folder with anything in it, since it may be
// the only copy; an empty one holds nothing, so the link just replaces it.
func inTheWay(name, source, path string, fi fs.FileInfo) Step {
	keep := func(why string) Step { return Step{Name: name, Action: Keep, Source: source, Why: why} }
	if !fi.IsDir() {
		return keep("a real file is in the way")
	}
	empty, err := isEmptyDir(path)
	switch {
	case err != nil:
		return keep("cannot list the folder in the way: " + reason(err))
	case empty:
		return Step{Name: name, Action: Link, Source: source, Why: "an empty folder is replaced by the link"}
	}
	return keep("a real folder is in the way")
}

func isEmptyDir(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	if _, err := f.Readdirnames(1); err == io.EOF {
		return true, nil
	} else if err != nil {
		return false, err
	}
	return false, nil
}

// staleLinks finds links in an account folder that clalt could have made (they
// aim at a direct child of the shared folder) but that no longer match a shared
// name. links aimed anywhere else belong to the user and are not touched.
func staleLinks(dir, sharedDir, prefix string, known map[string]bool) ([]Step, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Step
	for _, e := range entries {
		name := e.Name()
		if known[name] || e.Type()&fs.ModeSymlink == 0 {
			continue
		}
		path := filepath.Join(dir, name)
		target, err := linkTarget(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !fsutil.SamePath(filepath.Dir(target), sharedDir) {
			continue
		}
		why := "no longer a shared name"
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			why = "its source is gone"
		}
		out = append(out, Step{Name: prefix + name, Action: Remove, Old: target, Why: why})
	}
	return out, nil
}

// linkTarget returns where a symlink points as a cleaned absolute path. a
// relative target is read from the link's own folder, resolved first because
// ".." follows the real folder, not the spelling used to reach it.
func linkTarget(path string) (string, error) {
	t, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(t) {
		t = filepath.Join(fsutil.ResolvePath(filepath.Dir(path)), t)
	}
	return filepath.Clean(t), nil
}

// pointsAt is true when target is the shared home entry itself. a link that
// skips the shared home and aims at what that entry points to does not count:
// it would not follow the user re-pointing the entry.
func pointsAt(target, source string) bool {
	return filepath.Base(target) == filepath.Base(source) &&
		fsutil.SamePath(filepath.Dir(target), filepath.Dir(source))
}
