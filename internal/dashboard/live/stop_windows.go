//go:build windows

package live

import "context"

// the process table of Windows has no start time or parent clalt can compare
// the way ps allows, so a stop is refused instead of guessed
func probeProc(context.Context, int) (Proc, error) { return Proc{}, errNoStop }

func signalPID(int) error { return errNoStop }
