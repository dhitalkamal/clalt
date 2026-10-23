package profile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// maxAdoptSize is the largest file that is adopted. these files are small json
// documents, so anything bigger is not what claude wrote there.
const maxAdoptSize = 32 << 20

const adoptWhy = "claude replaced the link with a real file"

// adoptStep is the step for a real file where a shared plugins file belongs, or
// false when the file is not one to adopt: not in the list, not a regular file,
// or too big (on either side).
func adoptStep(dir, prefix, name, source string, fi fs.FileInfo) (Step, bool) {
	if prefix != "plugins/" || !adoptable[name] || !fi.Mode().IsRegular() || fi.Size() > maxAdoptSize {
		return Step{}, false
	}
	if !adoptLayout(filepath.Dir(dir)) {
		return Step{Name: prefix + name, Action: Keep, Source: source, Why: "a real file is in the way, and the account is not under a clalt accounts folder, so nothing is adopted"}, true
	}
	// the shared copy must be a plain file itself: a link there is the user's
	if sfi, err := os.Lstat(source); err != nil || !sfi.Mode().IsRegular() || sfi.Size() > maxAdoptSize {
		return Step{Name: prefix + name, Action: Keep, Source: source, Why: "a real file is in the way, and the shared copy is not a plain file, so nothing is adopted"}, true
	}
	return Step{Name: prefix + name, Action: Adopt, Source: source, Why: adoptWhy}, true
}

// adoptMissing adds an Adopt for each shareable plugins file that the account
// has as a real file while the shared folder has none, so the first copy is not
// lost to the account.
func adoptMissing(steps []Step, dir, sharedDir, prefix string, v sharedView) ([]Step, error) {
	if dir == "" || prefix != "plugins/" || !adoptLayout(filepath.Dir(dir)) {
		return steps, nil
	}
	for name := range adoptable {
		if _, there := v.link[name]; there {
			continue
		}
		if _, there := v.unreadable[name]; there {
			continue
		}
		source := filepath.Join(sharedDir, name)
		if _, err := os.Lstat(source); !errors.Is(err, fs.ErrNotExist) {
			continue
		}
		fi, err := os.Lstat(filepath.Join(dir, name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !fi.Mode().IsRegular() || fi.Size() > maxAdoptSize {
			continue
		}
		steps = append(steps, Step{Name: prefix + name, Action: Adopt, Source: source, Why: adoptWhy})
	}
	return steps, nil
}
