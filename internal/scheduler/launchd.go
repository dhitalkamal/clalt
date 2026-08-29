package scheduler

import (
	"fmt"

	"github.com/dhitalkamal/clalt/internal/launchd"
)

// launchdSched is the macOS backend, delegating to the launchd package.
type launchdSched struct{}

func (launchdSched) Name() string { return "launchd" }

// agent maps a Job onto a launchd.Agent.
func (launchdSched) agent(j Job) launchd.Agent {
	return launchd.Agent{
		Label:      j.Label,
		Program:    j.Program,
		Args:       j.Args,
		Interval:   int(j.Interval.Seconds()),
		StdoutPath: j.StdoutPath,
		StderrPath: j.StderrPath,
		RunAtLoad:  j.RunAtLoad,
	}
}

func (s launchdSched) Install(j Job) error {
	return launchd.Install(s.agent(j))
}

func (s launchdSched) Plan(j Job) string {
	return fmt.Sprintf("launchd: write %s\n\n%s\nthen: launchctl unload/load the plist",
		launchd.PlistPath(j.Label), s.agent(j).Plist())
}

func (launchdSched) Uninstall(label string) error { return launchd.Uninstall(label) }

func (launchdSched) Status(label string) (bool, error) { return launchd.Status(label) }

func (launchdSched) Location(label string) string { return launchd.PlistPath(label) }
