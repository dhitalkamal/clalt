package profile

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/dhitalkamal/clalt/internal/fslock"
	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// Adoption is a real file that claude saved over a link and that the sync took
// back. Backup is where the older copy went, empty when the shared copy was
// missing and there was nothing to keep.
type Adoption struct {
	Name   string
	Backup string
}

// backupDir is where the older copy of an adopted file goes: next to the
// accounts folder, never in the shared home and never where claude writes. an
// account folder <clalt dir>/accounts/<id> keeps its backups in
// <clalt dir>/backups/plugins/<id>.
func backupDir(profileDir string) string {
	return filepath.Join(claltDir(profileDir), "backups", "plugins", filepath.Base(profileDir))
}

// claltDir is the clalt folder an account folder sits in.
func claltDir(profileDir string) string { return filepath.Dir(filepath.Dir(profileDir)) }

// adoptLayout is true for an account folder that sits directly in an accounts
// (or, in the older layout, profiles) folder. anything else has no known place
// for backups, so nothing is adopted there rather than guessing one.
func adoptLayout(profileDir string) bool {
	parent := filepath.Base(filepath.Dir(profileDir))
	return parent == "accounts" || parent == "profiles"
}

// adoptLockTimeout is a var so a test can shorten it.
var adoptLockTimeout = 10 * time.Second

// unchanged is true while the file at path still looks like fi: claude saved
// to it in the meantime otherwise. mtime and size are all that can be checked,
// so a save that keeps both inside the last moments before the rename is the
// remaining window.
func unchanged(path string, fi fs.FileInfo) bool {
	now, err := os.Lstat(path)
	return err == nil && now.Mode().IsRegular() && now.Size() == fi.Size() && now.ModTime().Equal(fi.ModTime())
}

// adopt applies an Adopt step: the newer of the account's file and the shared
// one ends up at the shared path, the older one is backed up, and the account
// name becomes a link. done is false when the name is gone already. a zero
// Adoption with done true means both copies were the same, nothing to report.
func adopt(profileDir string, st Step) (ad Adoption, done bool, err error) {
	if err := checkParent(profileDir, st); err != nil {
		return ad, false, err
	}
	if !adoptLayout(profileDir) {
		return ad, false, errors.New("the account is not under a clalt accounts folder, nothing is adopted")
	}
	// the shared file is common to every account, so adopts of different
	// accounts must not interleave their read, compare and write
	l, err := fslock.Acquire(filepath.Join(claltDir(profileDir), "locks", "plugins-adopt.lock"), adoptLockTimeout)
	if err != nil {
		return ad, false, err
	}
	defer func() { _ = l.Release() }()
	path := filepath.Join(profileDir, filepath.FromSlash(st.Name))
	fi, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return ad, false, nil
	case err != nil:
		return ad, false, err
	case !fi.Mode().IsRegular() || fi.Size() > maxAdoptSize:
		return ad, false, errors.New("not a plain file of a sane size any more, left alone")
	}
	mine, err := fsutil.ReadFile(path)
	if err != nil {
		return ad, false, err
	}
	sfi, err := os.Lstat(st.Source)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return ad, false, err
	}
	var theirs []byte
	if err == nil {
		if !sfi.Mode().IsRegular() || sfi.Size() > maxAdoptSize {
			return ad, false, errors.New("the shared copy is not a plain file of a sane size, left alone")
		}
		if theirs, err = fsutil.ReadFile(st.Source); err != nil {
			return ad, false, err
		}
	}
	if !unchanged(path, fi) {
		return ad, false, errors.New("changed while being adopted, left alone")
	}
	base := filepath.Base(path)
	dir := backupDir(profileDir)
	switch {
	case sfi != nil && bytes.Equal(mine, theirs):
		// same content: only the link is missing
	case sfi == nil || fi.ModTime().After(sfi.ModTime()):
		perm := os.FileMode(0o600)
		if sfi != nil {
			if ad.Backup, err = backup(dir, base, theirs); err != nil {
				return ad, false, err
			}
			perm = sfi.Mode().Perm()
		}
		if err := fsutil.WriteFileAtomic(st.Source, mine, perm); err != nil {
			return ad, false, err
		}
		ad.Name = st.Name
	default:
		if ad.Backup, err = backup(dir, base, mine); err != nil {
			return ad, false, err
		}
		ad.Name = st.Name
	}
	if !unchanged(path, fi) {
		return ad, false, errors.New("changed while being adopted, left alone (both copies are kept, the next run adopts again)")
	}
	if _, err := replaceFileWithLink(st.Source, path); err != nil {
		return ad, false, err
	}
	return ad, true, nil
}

// backup saves data under dir as <base>.<unix time>, with a counter added when
// that name is taken, and returns the path.
func backup(dir, base string, data []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("make the backup folder: %w", err)
	}
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	for i := 0; i < 1000; i++ {
		name := base + "." + stamp
		if i > 0 {
			name += "." + strconv.Itoa(i)
		}
		p := filepath.Join(dir, name)
		if _, err := os.Lstat(p); err == nil {
			continue
		}
		return p, fsutil.WriteFileAtomic(p, data, 0o600)
	}
	return "", errors.New("no free name for the backup")
}

// replaceFileWithLink renames a new link over the real file at path, so the name
// never goes missing.
func replaceFileWithLink(source, path string) (bool, error) {
	tmp, err := tempLink(source, path)
	if err != nil {
		return false, err
	}
	if err := rename(tmp, path); err != nil {
		os.Remove(tmp)
		return false, err
	}
	return true, nil
}
