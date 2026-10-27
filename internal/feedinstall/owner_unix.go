//go:build !windows

package feedinstall

import (
	"os"
	"syscall"
)

func fileOwnedByUser(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	return !ok || int(st.Uid) == os.Getuid()
}
