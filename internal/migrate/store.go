package migrate

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// store is the shared folder as the copy code sees it. *os.Root is the real
// one: it refuses any name that leaves the folder, even through a link swapped
// in after a check. tests wrap it to watch or break a step.
type store interface {
	Lstat(name string) (fs.FileInfo, error)
	Open(name string) (*os.File, error)
	OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error)
	MkdirAll(name string, perm fs.FileMode) error
	Link(oldname, newname string) error
	Chtimes(name string, atime, mtime time.Time) error
	Remove(name string) error
}

// open returns the shared folder opened as a root. when the folder does not
// exist yet it returns nil, unless create is set, which makes it first.
func (c *copier) open(create bool) (store, error) {
	if c.st != nil {
		return c.st, nil
	}
	if create {
		if err := os.MkdirAll(c.destReal, 0o700); err != nil {
			return nil, err
		}
	}
	root, err := os.OpenRoot(c.destReal)
	if errors.Is(err, fs.ErrNotExist) && !create {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.root, c.st = root, root
	if c.wrap != nil {
		c.st = c.wrap(root)
	}
	return c.st, nil
}

func (c *copier) close() {
	if c.root != nil {
		_ = c.root.Close()
	}
}

// writeInPlace creates rel only if it does not exist yet and fills it from r. a
// failure after the create removes the partial file, since this is the one path
// where it would sit under its real name.
func (c *copier) writeInPlace(st store, rel string, r io.Reader, mtime time.Time) (int64, error) {
	f, err := st.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, err // not ours to remove
	}
	n, err := fill(f, r)
	if err == nil {
		err = st.Chtimes(rel, mtime, mtime)
	}
	if err != nil {
		err = joinRemove(err, st.Remove(rel), "could not remove the partial copy "+c.abs(rel)+", delete it by hand")
	}
	return n, err
}

// joinRemove adds the reason a leftover file could not be removed to err, so a
// cleanup problem is never lost. a file that is already gone is fine.
func joinRemove(err, rerr error, msg string) error {
	if rerr == nil || errors.Is(rerr, fs.ErrNotExist) {
		return err
	}
	return errors.Join(err, fmt.Errorf("%s: %w", msg, rerr))
}

func (c *copier) abs(rel string) string { return filepath.Join(c.destReal, rel) }

// fill copies r into f and closes it. the sync comes first: without it a crash
// could leave a short file under the final name, and a rerun would then call it
// a conflict for good.
func fill(f *os.File, r io.Reader) (int64, error) {
	n, err := io.Copy(f, r)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return n, err
}
