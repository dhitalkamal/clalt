//go:build windows

package crew

import "os/exec"

// detach does nothing here: clalt crew is not available on windows, and this
// file only keeps the package compiling there.
func detach(cmd *exec.Cmd) {}
