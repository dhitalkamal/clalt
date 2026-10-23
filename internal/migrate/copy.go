package migrate

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// tempPrefix starts the name of a file that is still being written. it has no
// .jsonl suffix, so claude never mistakes a leftover for a transcript.
const tempPrefix = ".clalt-migrate-"

// errLostRace means the final name appeared while copying and was gone again at
// the second look, so there is nothing there to judge.
var errLostRace = errors.New("destination appeared while copying")

type result int

const (
	_ result = iota
	placed
	identical
	differs // something is there that is not the same file, and it was left alone
)

type placement struct {
	res     result
	reason  Reason
	bytes   int64
	inPlace bool
}

type copier struct {
	dest     string
	destReal string // dest with links resolved, what every write must stay inside
	dry      bool
	wrap     func(store) store
	openSrc  func(path string) (*os.File, error)
	root     *os.Root
	st       store
	noLinks  bool // the filesystem cannot hard link, so files are written in place

	// dry run only: where each path would be placed from
	planned map[string]string
}

func (c *copier) file(p *Profile, root, src string) {
	rel, _ := filepath.Rel(root, src)
	name := filepath.ToSlash(rel)
	if err := c.contain(filepath.Dir(filepath.Join(c.dest, rel))); err != nil {
		p.fail(KindFile, name, err)
		return
	}
	pl, err := c.place(src, rel)
	switch pl.res {
	case placed:
		p.Copied++
		p.Bytes += pl.bytes
		if pl.inPlace {
			p.InPlace++
		}
	case identical:
		p.AlreadyThere++
	case differs:
		p.Conflicts = append(p.Conflicts, Conflict{Path: name, Reason: pl.reason})
	}
	if err != nil {
		p.fail(KindFile, name, err)
	}
}

// place copies src to rel in the shared folder, or leaves it alone when
// anything is there.
func (c *copier) place(src, rel string) (placement, error) {
	if pl, found, err := c.existing(src, rel); found || err != nil {
		return pl, err
	}
	if c.dry {
		return c.plan(src, rel)
	}
	out, err := c.publish(src, rel)
	switch {
	case out.lostRace:
		// look again and judge what won, never replace it
		pl, found, jerr := c.existing(src, rel)
		if jerr == nil && !found {
			jerr = errLostRace
		}
		return pl, errors.Join(err, jerr)
	case out.placed:
		return placement{res: placed, bytes: out.bytes, inPlace: out.inPlace}, err
	}
	return placement{}, err
}

// existing compares what sits at rel in the shared folder with src.
func (c *copier) existing(src, rel string) (pl placement, found bool, err error) {
	st, err := c.open(false)
	if err != nil || st == nil {
		return placement{}, false, err
	}
	info, err := st.Lstat(rel)
	if errors.Is(err, fs.ErrNotExist) {
		return placement{}, false, nil
	}
	if err != nil {
		return placement{}, false, err
	}
	// a folder or a link in the way is never followed, so it can never be
	// called identical
	if !info.Mode().IsRegular() {
		return placement{res: differs, reason: ReasonInTheWay}, true, nil
	}
	a, err := c.openSrc(src)
	if err != nil {
		return placement{}, true, err
	}
	defer a.Close()
	b, err := st.Open(rel)
	if err != nil {
		return placement{}, true, err
	}
	defer b.Close()
	res, reason, err := compareOpen(a, b)
	return placement{res: res, reason: reason}, true, err
}
