//go:build !windows

package fakeutil

import (
	"strconv"
	"syscall"
	"time"
)

// KillSelf ends the process by the named signal (TERM, KILL or HUP), as a kill
// from outside would, so the parent sees a signal death.
func KillSelf(name string) {
	sig := map[string]syscall.Signal{"TERM": syscall.SIGTERM, "KILL": syscall.SIGKILL, "HUP": syscall.SIGHUP}[name]
	_ = syscall.Kill(syscall.Getpid(), sig)
	time.Sleep(5 * time.Second)
}

// NotePgid records the process group the fake found itself in.
func NotePgid(path string) { WriteFile(path, strconv.Itoa(syscall.Getpgrp())+"\n") }
