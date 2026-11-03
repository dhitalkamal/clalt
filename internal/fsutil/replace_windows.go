package fsutil

import (
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// the FILE_RENAME_INFO_EX flags: replace the target, and do it with posix
// semantics, so a target that readers hold open (with FILE_SHARE_DELETE, see
// openShared) is swapped out under them instead of refusing the rename.
const (
	renameReplaceIfExists = 0x1
	renamePosixSemantics  = 0x2
)

// fileRenameInfo is FILE_RENAME_INFO with the Flags member of the union. the
// name follows the struct in the same buffer.
type fileRenameInfo struct {
	Flags          uint32
	RootDirectory  windows.Handle
	FileNameLength uint32
	FileName       [1]uint16
}

// replaceFile renames from over to. os.Rename uses MoveFileEx, which refuses
// to replace a file that is open anywhere; a posix rename does not. file
// systems without it (FAT, some network shares, windows before 10 1607) get
// os.Rename, so nothing gets worse there.
func replaceFile(from, to string) error {
	if err := posixRename(from, to); err == nil {
		return nil
	}
	return os.Rename(from, to)
}

func posixRename(from, to string) error {
	abs, err := filepath.Abs(to)
	if err != nil {
		return err
	}
	name, err := windows.UTF16FromString(abs)
	if err != nil {
		return err
	}
	src, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(src, windows.DELETE|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)

	var info fileRenameInfo
	nameBytes := (len(name) - 1) * 2 // without the trailing nul
	size := int(unsafe.Offsetof(info.FileName)) + len(name)*2
	// uint64 words keep the buffer aligned for the handle inside it
	buf := make([]uint64, (size+7)/8)
	p := (*fileRenameInfo)(unsafe.Pointer(&buf[0]))
	p.Flags = renameReplaceIfExists | renamePosixSemantics
	p.FileNameLength = uint32(nameBytes)
	copy(unsafe.Slice(&p.FileName[0], len(name)), name)
	return windows.SetFileInformationByHandle(h, windows.FileRenameInfoEx, (*byte)(unsafe.Pointer(p)), uint32(size))
}
