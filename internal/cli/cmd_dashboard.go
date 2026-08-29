package cli

import (
	"os"

	"golang.org/x/term"

	"github.com/dhitalkamal/clalt/internal/tui"
)

func (a *App) cmdDashboard(args []string) error {
	live, _ := parseUsageArgs(args)

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

	act, err := tui.RunDashboard(data)
	if err != nil {
		return err
	}
	// perform the chosen action now that the alt-screen is torn down
	switch act.Kind {
	case "switch":
		return a.cmdSwitch([]string{act.Account})
	case "run":
		return a.cmdRun([]string{act.Account})
	case "rotate":
		return a.cmdNext(nil)
	case "remove":
		return a.cmdRemove([]string{act.Account, "--force"}) // already confirmed in the TUI
	}
	return nil
}
