package cli

import (
	"context"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/dhitalkamal/clalt/internal/dashboard"
	"github.com/dhitalkamal/clalt/internal/dashboard/demo"
	"github.com/dhitalkamal/clalt/internal/tui"
)

// cmdDashboard opens the persistent multi-panel dashboard. In-place management
// runs through the tui.Controller (this App); actions that need the real
// terminal/browser (run/login/add) tear the alt-screen down, run here, then the
// loop re-opens the dashboard.
func (a *App) cmdDashboard(args []string) error {
	live, preview, err := parseDashboardArgs(args)
	if err != nil {
		return err
	}
	if preview {
		return a.cmdDashboardDemo(term.IsTerminal(int(os.Stdout.Fd())))
	}

	for {
		data, err := a.buildDashboardData(live)
		if err != nil {
			return err
		}
		if len(data.Accounts) == 0 {
			a.printf("no accounts to report on. run 'clalt add <name>' or 'clalt login <name>'.\n")
			return nil
		}

		// no TTY (piped / CI): render a single static frame instead of the TUI
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			a.printf("%s\n", tui.Render(data, 100, 40))
			return nil
		}

		act, err := tui.RunDashboard(a, data, live)
		if err != nil {
			return err
		}
		switch act.Kind {
		case "run":
			a.reportDashAction(a.cmdRun([]string{act.Account}), "run")
		case "login":
			a.reportDashAction(a.cmdLogin([]string{act.Account}), "login")
		case "add":
			a.reportDashAction(a.cmdAdd([]string{act.Account}), "add")
		default:
			return nil // quit
		}
		// loop: rebuild data and re-open the dashboard
	}
}

// reportDashAction prints a teardown action's error (if any) before the dashboard
// re-opens, so a failed run/login/add is not silently swallowed.
func (a *App) reportDashAction(err error, verb string) {
	if err != nil {
		a.printf("%s failed: %v\n", verb, err)
	}
}

const dashboardUsage = "clalt dashboard [--live | --demo]"

// parseDashboardArgs reads --live and --demo. --demo previews the new
// dashboard on made up data, the old one stays the default. any other flag is
// a mistake, and so is --live with --demo, which would otherwise be ignored.
func parseDashboardArgs(args []string) (live, preview bool, err error) {
	for _, arg := range args {
		switch {
		case arg == "--live":
			live = true
		case arg == "--demo":
			preview = true
		case strings.HasPrefix(arg, "-"):
			return false, false, usageError{"unknown flag " + arg, dashboardUsage}
		}
	}
	if live && preview {
		return false, false, usageError{"--live has no effect with --demo", dashboardUsage}
	}
	return live, preview, nil
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
