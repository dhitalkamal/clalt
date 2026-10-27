//go:build windows

package fakeutil

import "os"

// KillSelf ends with the code a shell reports for the signal, as windows has
// no signal to die by.
func KillSelf(name string) {
	os.Exit(map[string]int{"TERM": 143, "KILL": 137, "HUP": 129}[name])
}

// NotePgid has nothing to record: there are no process groups here.
func NotePgid(string) {}
