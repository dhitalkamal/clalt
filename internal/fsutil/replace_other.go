//go:build !windows

package fsutil

import "os"

// replaceFile is os.Rename: other systems replace a file that is open.
func replaceFile(from, to string) error { return os.Rename(from, to) }
