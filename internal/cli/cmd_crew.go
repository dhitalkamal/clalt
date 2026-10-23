package cli

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/dhitalkamal/clalt/internal/crew"
)

const crewUsage = "clalt crew <start|stop|status|logs> <name>"

func (a *App) crewManager() *crew.Manager { return crew.NewManager(a.crewOpts) }

// cmdCrew runs the Kiro Crew gateways, one per account. clalt only ever deals
// with the gateways it started itself, see docs/crew.md.
func (a *App) cmdCrew(args []string) error {
	if runtime.GOOS == "windows" {
		return errors.New("clalt crew is not available on Windows")
	}
	if len(args) == 0 {
		return usageError{"which crew command?", crewUsage}
	}
	switch args[0] {
	case "start":
		return a.crewStart(args[1:])
	case "stop":
		return a.crewStop(args[1:])
	case "status":
		return a.crewStatus(args[1:])
	case "logs":
		return a.crewLogs(args[1:])
	}
	return usageError{"unknown crew command " + args[0], crewUsage}
}

// crewTarget finds the account a stop, status or logs is about. a removed
// account counts, and so does an id that has a gateway folder but no account
// any more, so that a gateway is never left without a way to stop it.
func (a *App) crewTarget(label string) (id, name string, err error) {
	acc, found, err := a.Store.ResolveLabel(label)
	switch {
	case err != nil:
		return "", "", err
	case found:
		return acc.Name, shown(acc), nil
	}
	if _, has, err := crew.ReadRecord(label); err == nil && has {
		return label, clean(label), nil
	}
	return "", "", fmt.Errorf("no account called %q", label)
}

// crewLabels maps account ids to labels for printing.
func (a *App) crewLabels() map[string]string {
	labels, err := a.labelsByID()
	if err != nil {
		return map[string]string{}
	}
	return labels
}

func labelFor(labels map[string]string, id string) string {
	if l, ok := labels[id]; ok {
		return l
	}
	return clean(id)
}
