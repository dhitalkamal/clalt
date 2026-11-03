package fsutil

import (
	"time"
)

// openRead is a seam for tests, like rename.
var openRead = readShared

// ReadFile is os.ReadFile that waits out a rename in progress. on Windows an
// open fails with a sharing violation while another process is replacing the
// file, which WriteFileAtomic does to every state file. the wait is bounded, so
// a real permission problem still comes back within a fraction of a second.
func ReadFile(path string) ([]byte, error) {
	var (
		data []byte
		err  error
	)
	for i := 0; i < renameAttempts; i++ {
		data, err = openRead(path)
		if err == nil || !retryRename || !renameBusy(err) {
			return data, err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return data, err
}
