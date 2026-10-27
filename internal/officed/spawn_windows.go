//go:build windows

package officed

import "errors"

// Spawn is not available on Windows: the background office is out of scope
// there, Manager.On refuses before it gets here.
func Spawn(exe string, args []string, logPath string) (Child, error) {
	return Child{}, errors.New("the background office is not available on Windows yet")
}
