//go:build windows

package claudecli

import "os/exec"

// ownGroup does nothing here. a console interrupt on windows is not delivered
// by process group the way it is on unix, and this is left as it is.
func ownGroup(cmd *exec.Cmd) {}
