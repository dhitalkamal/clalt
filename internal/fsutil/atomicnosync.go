package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomicNoSync is WriteFileAtomic without the fsyncs: a reader sees the
// old or the new content, never a torn write, but a power cut may lose the
// write. for state that is rebuilt anyway (the session feed), where an fsync
// costs about 9 ms on macOS and the writer sits on a hook's critical path.
func WriteFileAtomicNoSync(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmpPath, perm)
	}
	if werr == nil {
		werr = renameWithRetry(tmpPath, path)
	}
	if werr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write %s: %w", filepath.Base(path), werr)
	}
	return nil
}
