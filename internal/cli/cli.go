package cli

import (
	"errors"
	"fmt"
	"strings"
)

// Run parses args (os.Args[1:]) and dispatches a subcommand, returning an exit code.
func Run(args []string) int {
	return run(NewApp(), args)
}

// usageError is a mistake in the arguments of a command. it exits with 2 and
// shows how the command is called.
type usageError struct{ msg, usage string }

func (e usageError) Error() string { return e.msg }

// splitJSONFlag takes the global --json flag out of args. what follows a double
// dash is for claude (`clalt run work -- ...`) and is left alone, and so is
// everything after the account that run names, since claude has a --json of its
// own and the README says that what follows the name goes to claude.
func splitJSONFlag(args []string) (rest []string, jsonOut bool) {
	command, named := "", false
	for i, a := range args {
		switch {
		case a == "--":
			return append(rest, args[i:]...), jsonOut
		case named:
			rest = append(rest, a)
		case a == "--json":
			jsonOut = true
		default:
			switch {
			case command == "" && !strings.HasPrefix(a, "-"):
				command = a
			case command == "run" && !strings.HasPrefix(a, "-"):
				named = true
			}
			rest = append(rest, a)
		}
	}
	return rest, jsonOut
}

func run(app *App, args []string) int {
	rest, jsonOut := splitJSONFlag(args)
	var err error
	app.JSON = jsonOut
	app.writeErr = nil

	if len(rest) == 0 {
		app.usage()
		return app.exitCode(app.finish(nil))
	}

	cmd, cmdArgs := rest[0], rest[1:]
	if err := goneUse(cmd, cmdArgs); err != nil {
		return app.exitCode(err)
	}
	switch cmd {
	case "version", "--version", "-v":
		err = app.cmdVersion(cmdArgs)
	case "list", "ls":
		err = app.cmdList(cmdArgs)
	case "whoami", "current":
		err = app.cmdWhoami(cmdArgs)
	case "run":
		err = app.cmdRun(cmdArgs)
	case "budget":
		err = app.cmdBudget(cmdArgs)
	case "sessions":
		err = app.cmdSessions(cmdArgs)
	case "resume":
		err = app.cmdResume(cmdArgs)
	case "launch":
		err = app.cmdLaunch(cmdArgs)
	case "jump":
		err = app.cmdJump(cmdArgs)
	case "notify":
		err = app.cmdNotify(cmdArgs)
	case "remove", "rm":
		err = app.cmdRemove(cmdArgs)
	case "rename":
		err = app.cmdRename(cmdArgs)
	case "login":
		err = app.cmdLogin(cmdArgs)
	case "logout":
		err = app.cmdLogout(cmdArgs)
	case "doctor":
		err = app.cmdDoctor(cmdArgs)
	case "usage":
		err = app.cmdUsage(cmdArgs)
	case "dashboard":
		err = app.cmdDashboard(cmdArgs)
	case "migrate":
		err = app.cmdMigrate(cmdArgs)
	case "rules":
		err = app.cmdRules(cmdArgs)
	case "move":
		err = app.cmdMove(cmdArgs)
	case "hook":
		err = app.cmdHook(cmdArgs)
	case "statusline":
		err = app.cmdStatusline(cmdArgs)
	case "crew":
		err = app.cmdCrew(cmdArgs)
	case "help", "-h", "--help":
		app.usage()
	default:
		fmt.Fprintf(app.Err, "unknown command %q (try 'clalt help')\n", cmd)
		return 2
	}

	return app.exitCode(app.finish(err))
}

// exitCode prints err the way the command line promises and returns the exit
// code: claude's own when it was a session that ended, 2 for a mistake in the
// arguments or a retired command, 1 for anything else that went wrong.
func (a *App) exitCode(err error) int {
	var bad usageError
	var gone goneError
	var exit exitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		return exit.code
	case errors.As(err, &gone):
		fmt.Fprintf(a.Err, "error: %s\n", clean(gone.msg))
		return 2
	case errors.As(err, &bad):
		fmt.Fprintf(a.Err, "error: %s\nusage: %s\n", clean(bad.msg), clean(bad.usage))
		return 2
	}
	fmt.Fprintf(a.Err, "error: %s\n", clean(err.Error()))
	return 1
}

func (a *App) usage() {
	a.printf(`clalt - run several Claude accounts side by side with Claude Code

usage:
  clalt login <name>          sign an account in through claude's own login (a
                              new name makes the account, a removed one comes back)
  clalt run <name> [-- ...]   run claude as <name>, in its own folder, with the
                              history shared with ~/.claude (accounts run in parallel)
  clalt run [-- ...]          run claude as the allowed account with the most room
                              left for this folder (rules: clalt rules)
  clalt logout <name>         sign the account out, keep its history
  clalt remove <name>         sign out and hide the account, history stays
  clalt remove <name> --delete  sign out, delete its folder and record
                              (--force skips the confirmation)
  clalt rename <old> <new>    change the label only, the folder never moves
  clalt list                  accounts with their live sign-in state
  clalt whoami                which account this shell is in
  clalt doctor                check claude, the accounts and leftovers of v0.1.0
                              (only reports, never changes anything)
  clalt migrate [--dry-run]   copy history from per-account profile folders into
                              the shared projects folder (safe to repeat)
  clalt rules <name>          show or set the folders an account may serve and
                              whether auto-switch may use it (--add-folder,
                              --remove-folder, --auto-switch, --no-auto-switch)
  clalt move <session> <account>
                              move a running session to another account at its
                              next pause (the session by handle, pid or id prefix)
  clalt sessions              list live parallel run sessions
  clalt resume [words]        list conversations of every account, newest first
                              (--limit n, --json)
  clalt resume <id> [--account <name>]
                              continue a conversation as an account, in the folder
                              it ran in
  clalt launch <name> [--cwd dir] [-- ...]
                              run claude as <name> in a new kitty tab or tmux window
  clalt jump <session>        bring the terminal of a running session to the front
  clalt notify [on|off|status|test]
                              alerts when a session needs you
  clalt crew start <name>     run a Kiro Crew gateway for the account (localhost only)
  clalt crew stop <name>      stop the gateway clalt started for the account
  clalt crew status [name]    the gateways clalt started: port, state, uptime
  clalt crew logs <name>      what the gateway printed (-n lines, default 100)
  clalt budget [name]         show or set per-account spend caps (--daily/--weekly/--clear)
  clalt usage [name]          local usage stats
  clalt dashboard             full-screen dashboard of accounts, sessions and usage
  clalt dashboard --demo      the dashboard on made-up data, touches no account
  clalt version               print build version (--json for machine-readable)

global:
  --json                      machine-readable output where supported
`)
}
