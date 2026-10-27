//go:build !windows

package live

import "syscall"

// nonBlock keeps an open from waiting for a writer
const nonBlock = syscall.O_NONBLOCK
