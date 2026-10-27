package migrate

import (
	"io/fs"
	"os"
	"testing"
	"time"
)

// faulty wraps the destination so a test can watch or break one step. A hook
// that is nil falls through to the real folder.
type faulty struct {
	store
	lstat    func(s store, name string) (fs.FileInfo, error)
	open     func(s store, name string) (*os.File, error)
	link     func(s store, oldname, newname string) error
	chtimes  func(s store, name string, atime, mtime time.Time) error
	mkdirAll func(s store, name string, perm fs.FileMode) error
	openFile func(s store, name string, flag int, perm fs.FileMode) (*os.File, error)
	remove   func(s store, name string) error
}

func (f faulty) Lstat(name string) (fs.FileInfo, error) {
	if f.lstat != nil {
		return f.lstat(f.store, name)
	}
	return f.store.Lstat(name)
}

func (f faulty) Open(name string) (*os.File, error) {
	if f.open != nil {
		return f.open(f.store, name)
	}
	return f.store.Open(name)
}

func (f faulty) Link(oldname, newname string) error {
	if f.link != nil {
		return f.link(f.store, oldname, newname)
	}
	return f.store.Link(oldname, newname)
}

func (f faulty) Chtimes(name string, atime, mtime time.Time) error {
	if f.chtimes != nil {
		return f.chtimes(f.store, name, atime, mtime)
	}
	return f.store.Chtimes(name, atime, mtime)
}

func (f faulty) MkdirAll(name string, perm fs.FileMode) error {
	if f.mkdirAll != nil {
		return f.mkdirAll(f.store, name, perm)
	}
	return f.store.MkdirAll(name, perm)
}

func (f faulty) OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error) {
	if f.openFile != nil {
		return f.openFile(f.store, name, flag, perm)
	}
	return f.store.OpenFile(name, flag, perm)
}

func (f faulty) Remove(name string) error {
	if f.remove != nil {
		return f.remove(f.store, name)
	}
	return f.store.Remove(name)
}

// withFaults is how a test hands a faulty destination to Run.
func withFaults(f faulty) Options {
	return Options{wrap: func(s store) store {
		f.store = s
		return f
	}}
}

// openDest opens a temp folder the way Run opens the shared one.
func openDest(t *testing.T) (*os.Root, string) {
	t.Helper()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root, dir
}
