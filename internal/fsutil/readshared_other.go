//go:build !windows

package fsutil

import "os"

// readShared is os.ReadFile: other systems let a file be replaced while it is
// open.
func readShared(path string) ([]byte, error) { return os.ReadFile(path) }
