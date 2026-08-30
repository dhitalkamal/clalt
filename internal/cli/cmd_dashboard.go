package cli

import (
	"os"

	"golang.org/x/term"

	"github.com/dhitalkamal/clalt/internal/tui"
)

// cmdDashboard opens the persistent multi-panel dashboard. In-place management
// runs through the tui.Controller (this App); actions that need the real
// terminal/browser (run/login/add) tear the alt-screen down, run here, then the
// loop re-opens the dashboard.
func (a *App) cmdDashboard(args []string) error {
	live, _ := parseUsageArgs(args)

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
