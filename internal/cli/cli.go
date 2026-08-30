package cli

import (
	"fmt"
	"os"
)

// Run parses args (os.Args[1:]) and dispatches a subcommand, returning an exit code.
func Run(args []string) int {
	var rest []string
	jsonOut := false
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
			continue
		}
		rest = append(rest, a)
	}

	app, err := NewApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	app.JSON = jsonOut

	if len(rest) == 0 {
		app.usage()
		return 0
	}

	cmd, cmdArgs := rest[0], rest[1:]
	switch cmd {
	case "list", "ls":
		err = app.cmdList()
	case "whoami", "current":
		err = app.cmdWhoami()
	case "add":
		err = app.cmdAdd(cmdArgs)
	case "run":
		err = app.cmdRun(cmdArgs)
	case "budget":
		err = app.cmdBudget(cmdArgs)
	case "sessions":
		err = app.cmdSessions(cmdArgs)
	case "remove", "rm":
		err = app.cmdRemove(cmdArgs)
	case "rename":
		err = app.cmdRename(cmdArgs)
	case "login":
		err = app.cmdLogin(cmdArgs)
	case "logout":
		err = app.cmdLogout(cmdArgs)
	case "refresh":
		err = app.cmdRefresh(cmdArgs)
	case "keepalive":
		err = app.cmdKeepalive(cmdArgs)
	case "doctor":
		err = app.cmdDoctor(cmdArgs)
	case "export":
		err = app.cmdExport(cmdArgs)
	case "import":
		err = app.cmdImport(cmdArgs)
	case "usage":
		err = app.cmdUsage(cmdArgs)
	case "dashboard":
		err = app.cmdDashboard(cmdArgs)
	case "help", "-h", "--help":
		app.usage()
		return 0
	default:
		fmt.Fprintf(app.Err, "unknown command %q (try 'clalt help')\n", cmd)
		return 2
	}

	if err != nil {
		fmt.Fprintf(app.Err, "error: %v\n", err)
		return 1
	}

	// once the user has an account, make sure background token keepalive is
	// running so they never have to set it up by hand. best-effort and quiet.
	switch cmd {
	case "login", "add", "run", "import":
		app.ensureKeepalive()
	}
	return 0
}

func (a *App) usage() {
	a.printf(`clalt - manage multiple Claude accounts for Claude Code

usage:
  clalt list                 list accounts (active one marked)
  clalt whoami               show the active account
  clalt add <name>           capture the current Claude Code login as <name>
  clalt login <name>         authenticate a new account via OAuth and store it
  clalt run <name> [-- ...]  run claude as <name>, keychain-free, in an isolated
                             profile (parallel: different accounts at once)
  clalt budget [name ...]    show or set per-account spend caps (--daily/--weekly)
  clalt sessions             list live parallel run sessions
  clalt remove <name>        delete an account slot (--force to skip confirm)
  clalt rename <old> <new>   rename an account
  clalt logout <name>        clear an account's stored credential
  clalt refresh <name>       refresh an account's token now
  clalt refresh --all        refresh every account's token now
  clalt keepalive            token refresh runs automatically in the background
                             once you have an account; 'install [--dry-run]',
                             'uninstall', 'status' manage it (launchd/systemd/schtasks)
  clalt doctor [--fix]       diagnose account tokens and vault state
  clalt export <file>        encrypted backup of all accounts
  clalt import <file>        restore accounts from a backup
  clalt usage [name] [--live]  usage stats (local by default)
  clalt dashboard [--live]     interactive usage dashboard (TUI)

global:
  --json                      machine-readable output where supported

note: --live queries Anthropic's usage endpoint with your token, a ToS gray
area (see README). local mode only reads files and is the default.
`)
}
