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
)

const dashboardUsage = "clalt dashboard [--demo]"

// cmdDashboard opens the dashboard on this machine's accounts, or on made up
// data with --demo. without a terminal it prints one plain frame instead.
func (a *App) cmdDashboard(args []string) error {
	preview, err := parseDashboardArgs(args)
	if err != nil {
		return err
	}
	interactive := a.interactive()
	if preview {
		return a.cmdDashboardDemo(interactive)
	}
	accts, err := a.visibleAccounts()
	if err != nil {
		return cleanErr(err)
	}
	if len(accts) == 0 {
		a.printf("no accounts to report on. sign one in with: clalt login <name>\n")
		return nil
	}
	ad := a.liveAdapter()
	if !interactive {
		snap, err := ad.Load(context.Background())
		if err != nil {
			return cleanErr(err)
		}
		a.printf("%s\n", dashboard.Render(snap, 100, 40, dashboard.Options{ASCII: true}))
		return nil
	}
	// matching plain claude to an account asks claude, which is slow: it runs
	// beside the dashboard, and the next load picks the answer up from the cache
	go a.refreshDefaultHomeIfStale()
	opt := dashboard.EnvOptions(os.Getenv)
	opt.Logger = dashboardLogger(dashboardLogPath())
	return dashboard.Run(context.Background(), ad, ad, opt)
}

// interactive says whether the program has a terminal to draw on and read
// keys from. it asks the app's own streams, which are the process's in a real
// run: asking os.Stdout started the program under go test on a terminal.
func (a *App) interactive() bool { return isTerminal(a.In) && isTerminal(a.Out) }

func isTerminal(x any) bool {
	f, ok := x.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// afterDashAction prints the error of a run or login that the dashboard started
// (if any) and waits for enter: the dashboard takes the screen back at once, and
// the reason of a refusal would be gone before it was read. a session that
// ended with a non-zero code is claude's business, not a failure here. with no
// input to read from there is nobody to wait for.
func (a *App) afterDashAction(err error, verb string) {
	var exit exitError
	if err == nil || errors.As(err, &exit) {
		return
	}
	fmt.Fprintf(a.Err, "%s failed: %s\n", verb, clean(err.Error()))
	_, _ = a.prompt("press enter to go back to the dashboard: ")
}

// parseDashboardArgs reads --demo, which shows the dashboard on made up data.
// any other flag is a mistake.
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

// cmdDashboardDemo runs the dashboard on demo data. it touches no account.
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
