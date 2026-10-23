package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/dhitalkamal/clalt/internal/dashboard"
	"github.com/dhitalkamal/clalt/internal/dashboard/demo"
	"github.com/dhitalkamal/clalt/internal/tui"
)

// cmdDashboard opens the persistent multi-panel dashboard. In-place management
// runs through the tui.Controller (this App); actions that need the real
// terminal or browser (run, login) tear the alt-screen down, run here, then the
// loop re-opens the dashboard.
func (a *App) cmdDashboard(args []string) error {
	preview, err := parseDashboardArgs(args)
	if err != nil {
		return err
	}
	interactive := term.IsTerminal(int(os.Stdout.Fd()))
	if preview {
		return a.cmdDashboardDemo(interactive)
	}

	// no TTY (piped / CI): render a single static frame instead of the TUI
	if !interactive {
		frame, err := a.dashboardFrame()
		if err != nil {
			return err
		}
		if frame == "" {
			a.printf("no accounts to report on. sign one in with: clalt login <name>\n")
			return nil
		}
		a.printf("%s\n", frame)
		return nil
	}

	for {
		data, err := a.buildDashboardData()
		if err != nil {
			return err
		}
		if len(data.Accounts) == 0 {
			a.printf("no accounts to report on. sign one in with: clalt login <name>\n")
			return nil
		}
		act, err := tui.RunDashboard(a, data)
		if err != nil {
			return err
		}
		switch act.Kind {
		case "run":
			a.afterDashAction(a.cmdRun([]string{act.Account}), "run")
		case "login":
			a.afterDashAction(a.cmdLogin([]string{act.Account}), "login")
		default:
			return nil // quit
		}
		// loop: rebuild data and re-open the dashboard
	}
}

// dashboardFrame is one static frame of the dashboard, or "" when there are no
// accounts to show.
func (a *App) dashboardFrame() (string, error) {
	data, err := a.buildDashboardData()
	if err != nil || len(data.Accounts) == 0 {
		return "", err
	}
	return tui.Render(data, 100, 40), nil
}

// afterDashAction prints a teardown action's error (if any) before the dashboard
// re-opens, and waits for enter: the dashboard takes the screen back at once,
// and the reason of a refused run or login would be gone before it was read. a
// session that ended with a non-zero code is claude's business, not a failure
// here. with no input to read from there is nobody to wait for.
func (a *App) afterDashAction(err error, verb string) {
	var exit exitError
	if err == nil || errors.As(err, &exit) {
		return
	}
	fmt.Fprintf(a.Err, "%s failed: %s\n", verb, clean(err.Error()))
	_, _ = a.prompt("press enter to go back to the dashboard: ")
}

const dashboardUsage = "clalt dashboard [--demo]"

// parseDashboardArgs reads --demo. --demo previews the new dashboard on made up
// data, the old one stays the default. any other flag is a mistake.
func parseDashboardArgs(args []string) (preview bool, err error) {
	for _, arg := range args {
		switch {
		case arg == "--demo":
			preview = true
		case strings.HasPrefix(arg, "-"):
			return false, usageError{"unknown flag " + arg, dashboardUsage}
		}
	}
	return preview, nil
}

// cmdDashboardDemo runs the new dashboard on demo data. it touches no account.
// without a terminal it prints one plain frame instead.
func (a *App) cmdDashboardDemo(interactive bool) error {
	now := demo.Clock()
	if !interactive {
		snap, err := demo.Source(now).Load(context.Background())
		if err != nil {
			return err
		}
		a.printf("%s\n", dashboard.Render(snap, 100, 30, dashboard.Options{ASCII: true, Now: now}))
		return nil
	}
	// no Logger: made up data has no failures worth keeping
	opt := dashboard.EnvOptions(os.Getenv)
	opt.Now = now
	return dashboard.Run(context.Background(), demo.Source(now), demo.Actions(), opt)
}
