//go:build windows

package crew

import "errors"

// terminate is not available here: clalt crew is not, and this file only keeps
// the package compiling on windows.
func terminate(pid int) error { return errors.New("clalt crew is not available on windows") }
