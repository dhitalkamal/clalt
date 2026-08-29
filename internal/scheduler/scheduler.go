// Package scheduler installs a periodic background job on the host OS behind a
// single interface, so clalt's keepalive can run on macOS (launchd), Linux
// (systemd user timer), and Windows (Task Scheduler). Each backend runs the job
// as a periodic one-shot (`clalt keepalive --once`) rather than a long-lived
// process, so a crash never leaves refresh silently dead.
package scheduler

import (
	"fmt"
	"math"
	"time"
)

// Job describes a periodic one-shot background job.
type Job struct {
	Label      string        // stable id, e.g. com.clalt.keepalive
	Program    string        // absolute path to the clalt binary
	Args       []string      // args after Program, e.g. keepalive --once
	Interval   time.Duration // time between runs
	StdoutPath string        // where the backend writes stdout (if supported)
	StderrPath string        // where the backend writes stderr (if supported)
	RunAtLoad  bool          // also run once immediately on install
}

// Scheduler installs and manages a Job on the host OS.
type Scheduler interface {
	// Name is the backend name (launchd, systemd, schtasks).
	Name() string
	// Install creates or replaces the job and starts its schedule.
	Install(Job) error
	// Uninstall stops and removes the job; missing is not an error.
	Uninstall(label string) error
	// Status reports whether the job is currently installed/enabled.
	Status(label string) (installed bool, err error)
	// Location returns a human-facing path or identifier for the job.
	Location(label string) string
	// Plan returns a human-readable description of exactly what Install would
	// write and run, without touching the system (for a dry run).
	Plan(Job) string
}

// For returns the Scheduler for the given GOOS, or an error if the platform has
// no supported background backend.
func For(goos string) (Scheduler, error) {
	switch goos {
	case "darwin":
		return launchdSched{}, nil
	case "linux":
		return systemdSched{}, nil
	case "windows":
		return taskschedSched{}, nil
	default:
		return nil, fmt.Errorf("no background scheduler backend for %s", goos)
	}
}

// intervalMinutes converts a Job interval to whole minutes for backends that
// only schedule by the minute (Task Scheduler), rounding to nearest and never
// returning less than 1.
func intervalMinutes(d time.Duration) int {
	m := int(math.Round(d.Minutes()))
	if m < 1 {
		return 1
	}
	return m
}
