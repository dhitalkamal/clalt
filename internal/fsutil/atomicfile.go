// Package fsutil provides low-level file helpers shared by every clalt
// package that persists state to disk, so a kill (or crash) mid-write can
// never leave a truncated or partially-written file behind.
package fsutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

// rename and retryRename are seams for tests: the retry only matters on Windows
// and cannot be provoked portably otherwise.
var rename = os.Rename
var retryRename = runtime.GOOS == "windows"

const renameAttempts = 10

// WriteFileAtomic writes data to path so a reader either sees the old
// content or the new content in full, never a torn write. It writes to a
// temp file in the same directory as path (so the later rename stays on one
// filesystem), fsyncs it, chmods it to perm, then renames it over path.
// Rename is atomic on both POSIX and Windows (Go's os.Rename uses
// MoveFileEx with MOVEFILE_REPLACE_EXISTING there), so a crash can only ever
// leave the old file or the fully-written new one at path.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	// clean up the temp file on any failure path below so a partial attempt
	// never litters the directory with an orphaned .tmp-* file.
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := renameWithRetry(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file into place: %w", err)
	}
	committed = true

	// fsync the directory too, so the rename entry itself is durable and not
	// just the file's bytes. Windows has no directory-fsync equivalent (and
	// does not need one: NTFS journals the rename), so skip it there.
	if runtime.GOOS != "windows" {
		if err := syncDir(dir); err != nil {
			return fmt.Errorf("sync directory: %w", err)
		}
	}
	return nil
}

// renameWithRetry retries a rename that fails because another process has the
// target open. Windows refuses to replace a file that a reader holds for a few
// milliseconds, and state.json is read by every clalt command. the wait is
// bounded, so a real permission problem still fails quickly.
func renameWithRetry(from, to string) error {
	var err error
	for i := 0; i < renameAttempts; i++ {
		err = rename(from, to)
		if err == nil || !retryRename || !renameBusy(err) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return err
}

// renameBusy matches access denied, plus the sharing and lock violations that
// Windows reports for a file held open elsewhere.
func renameBusy(err error) bool {
	if errors.Is(err, fs.ErrPermission) {
		return true
	}
	var errno syscall.Errno
	return runtime.GOOS == "windows" && errors.As(err, &errno) && (errno == 32 || errno == 33)
}

// syncDir fsyncs a directory so a rename inside it survives a crash, not just
// the renamed file's own contents.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
