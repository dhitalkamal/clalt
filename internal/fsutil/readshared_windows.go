package fsutil

import (
	"io"
	"io/fs"
	"os"
	"syscall"
)

// readShared reads a file opened with FILE_SHARE_DELETE. os.ReadFile leaves
// that share out, and windows refuses to replace a file while any handle lacks
// it, so every reader of a state file would block WriteFileAtomic. with it the
// rename goes through and the reader keeps the old content it opened.
func readShared(path string) ([]byte, error) {
	f, err := openShared(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// openShared opens path for reading with every share mode, delete included.
func openShared(path string) (*os.File, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: path, Err: err}
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		// a PathError keeps errors.Is(err, fs.ErrNotExist) and renameBusy working
		return nil, &fs.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}
