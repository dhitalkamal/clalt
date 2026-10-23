// Package migrate copies the conversation history held in per-account clalt
// profiles into the shared projects folder. it only reads the profiles and
// never overwrites anything already in the shared folder.
package migrate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Run looks at every entry directly under profilesDir and copies the history of
// each profile folder into dest. it scans the folders on disk, not state.json,
// because earlier renames orphaned some.
func Run(profilesDir, dest string, opts Options) (Report, error) {
	rep := Report{Source: profilesDir, Dest: dest, DryRun: opts.DryRun, Profiles: []Profile{}}
	if !filepath.IsAbs(profilesDir) || !filepath.IsAbs(dest) {
		return rep, fmt.Errorf("profiles folder %q and destination %q must both be absolute paths", profilesDir, dest)
	}
	entries, err := os.ReadDir(profilesDir)
	if errors.Is(err, fs.ErrNotExist) {
		return rep, nil
	}
	if err != nil {
		return rep, err
	}
	if st, err := os.Stat(dest); err == nil && !st.IsDir() {
		return rep, fmt.Errorf("%s is not a folder", dest)
	}
	destReal, err := ResolveExisting(dest)
	if err != nil {
		return rep, err
	}
	c := &copier{dest: dest, destReal: destReal, dry: opts.DryRun, wrap: opts.wrap, openSrc: opts.openSrc, planned: map[string]string{}}
	if c.openSrc == nil {
		c.openSrc = os.Open
	}
	defer c.close()
	for _, e := range entries {
		switch {
		case isLink(e.Type()):
			if p, ok := linkedProfile(profilesDir, e.Name()); ok {
				rep.Profiles = append(rep.Profiles, p)
			}
		case e.IsDir():
			rep.Profiles = append(rep.Profiles, c.scan(profilesDir, e.Name()))
		}
	}
	rep.Total = total(rep.Profiles)
	return rep, nil
}

func newProfile(name string) Profile {
	return Profile{
		Name:           name,
		Status:         StatusNothing,
		Conflicts:      []Conflict{},
		SkippedEntries: []Skip{},
		Failed:         []Failure{},
	}
}

func (c *copier) scan(profilesDir, name string) Profile {
	p := newProfile(name)
	projects := filepath.Join(profilesDir, name, "projects")
	info, err := os.Lstat(projects)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// no projects folder
	case err != nil:
		// not being able to look is not the same as there being nothing
		p.Status = StatusFolder
		p.fail(KindFolder, "projects", err)
	case isLink(info.Mode()):
		c.projectsLink(&p, projects)
	case info.IsDir() && samePlace(projects, c.dest):
		p.Status = StatusShared
	case info.IsDir():
		p.Status = StatusFolder
		c.walk(&p, projects)
	}
	return p
}

// projectsLink never follows the link into a copy. it only decides whether the
// link already leads to the shared folder.
func (c *copier) projectsLink(p *Profile, projects string) {
	target, why := linkTarget(projects)
	if why == "" && samePlace(target, c.dest) {
		p.Status = StatusShared
		return
	}
	p.Status, p.LinkOf, p.Target, p.Error = StatusLink, LinkOfProjects, target, why
}

// linkedProfile reports a profile entry that is a link. it is not followed, but
// it must not vanish from the report. a link to a plain file is no profile.
func linkedProfile(profilesDir, name string) (Profile, bool) {
	path := filepath.Join(profilesDir, name)
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		return Profile{}, false
	}
	p := newProfile(name)
	p.Status, p.LinkOf = StatusLink, LinkOfProfile
	p.Target, p.Error = linkTarget(path)
	return p, true
}

// linkTarget is where a link leads with every link resolved. when that fails it
// is the link as written, and why holds the reason.
func linkTarget(path string) (target, why string) {
	target, err := filepath.EvalSymlinks(path)
	if err == nil {
		return target, ""
	}
	raw, _ := os.Readlink(path)
	return raw, err.Error()
}

func (p *Profile) fail(kind, path string, err error) {
	p.Failed = append(p.Failed, Failure{Path: path, Kind: kind, Error: err.Error()})
}

func total(profiles []Profile) Totals {
	var t Totals
	for _, p := range profiles {
		t.Copied += p.Copied
		t.Bytes += p.Bytes
		t.InPlace += p.InPlace
		t.AlreadyThere += p.AlreadyThere
		t.Conflicts += len(p.Conflicts)
		t.Skipped += p.Skipped
		t.Failed += len(p.Failed)
		if p.Status == StatusLink {
			t.Linked++
		}
	}
	return t
}

// samePlace compares resolved locations, so a link routed through an alias
// still matches.
func samePlace(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

// isLink is true for a symlink and for a windows junction, which Go reports as
// irregular.
func isLink(mode fs.FileMode) bool {
	return mode&(fs.ModeSymlink|fs.ModeIrregular) != 0
}
