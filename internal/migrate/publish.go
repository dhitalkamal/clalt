package migrate

import (
	"crypto/rand"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// published is how a publish ended. lostRace means the final name appeared
// while we were copying, so it belongs to someone else now. an error can come
// with a placed file, when only the cleanup afterwards failed.
type published struct {
	placed   bool
	inPlace  bool
	lostRace bool
	bytes    int64
}

// publish writes src into a temp file beside rel and only then links it to its
// final name. unlike a rename, a link fails when rel already exists, and the
// final name never shows a half written file.
func (c *copier) publish(src, rel string) (published, error) {
	in, err := c.openSrc(src)
	if err != nil {
		return published{}, err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return published{}, err
	}
	st, err := c.open(true)
	if err != nil {
		return published{}, err
	}
	if err := st.MkdirAll(filepath.Dir(rel), 0o700); err != nil {
		return published{}, err
	}
	if c.noLinks {
		return c.writeDirect(st, rel, in, info.ModTime())
	}
	return c.viaTemp(st, rel, in, info.ModTime())
}

func (c *copier) viaTemp(st store, rel string, in *os.File, mtime time.Time) (out published, err error) {
	name := filepath.Join(filepath.Dir(rel), tempPrefix+rand.Text())
	tmp, err := st.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return published{}, err
	}
	// after a good link this only drops the temp name, rel keeps the data
	defer func() {
		err = joinRemove(err, st.Remove(name), "could not remove the temp file "+c.abs(name))
	}()

	n, err := fill(tmp, in)
	if err != nil {
		return published{}, err
	}
	// claude sorts its resume list by mtime
	if err := st.Chtimes(name, mtime, mtime); err != nil {
		return published{}, err
	}

	err = st.Link(name, rel)
	switch {
	case err == nil:
		return published{placed: true, bytes: n}, nil
	case errors.Is(err, fs.ErrExist):
		return published{lostRace: true}, nil
	case hardLinksUnsupported(err):
		// every later file goes straight to the in-place write, and this one is
		// written from the finished temp: the source may have grown since
		c.noLinks = true
		done, err := st.Open(name)
		if err != nil {
			return published{}, err
		}
		defer done.Close()
		return c.writeDirect(st, rel, done, mtime)
	}
	return published{}, err
}

// writeDirect writes the final name itself. it still never replaces anything,
// but unlike a link it is not atomic.
func (c *copier) writeDirect(st store, rel string, r io.Reader, mtime time.Time) (published, error) {
	n, err := c.writeInPlace(st, rel, r, mtime)
	switch {
	case errors.Is(err, fs.ErrExist):
		return published{lostRace: true}, nil
	case err != nil:
		return published{}, err
	}
	return published{placed: true, inPlace: true, bytes: n}, nil
}

// hardLinksUnsupported is true for the errors a filesystem gives when it cannot
// hard link at all (fat, exfat, some network shares), as opposed to a link that
// failed for a real reason.
func hardLinksUnsupported(err error) bool {
	return errors.Is(err, errors.ErrUnsupported) || errors.Is(err, syscall.EPERM)
}
