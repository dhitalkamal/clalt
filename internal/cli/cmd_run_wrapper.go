package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/claudecli"
	"github.com/dhitalkamal/clalt/internal/wrapper"
)

// runLauncher starts claude for the wrapper, with the terminal handed over and
// the session handle in its environment.
type runLauncher struct {
	a      *App
	handle string
}

var _ wrapper.Launcher = runLauncher{}

// Start runs claude in an account folder. a resume under another account comes
// through here too.
func (l runLauncher) Start(folder string, args []string) (wrapper.Process, error) {
	cmd, err := claudecli.Session(context.Background(), folder, args)
	if err != nil {
		return nil, err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = l.a.In, l.a.Out, l.a.Err
	cmd.Env = setEnv(cmd.Env, sessionEnvVar, l.handle)
	p, err := wrapper.StartExec(cmd)
	if err != nil {
		return nil, fmt.Errorf("start claude: %w", err)
	}
	return p, nil
}

// runWrapped runs the session under the wrapper: it ends with claude, and in
// between it can move the session to another account at a pause, see package
// wrapper. auto-switch is decided by the rules of each account and is off
// unless one was turned on with `clalt rules`.
func (a *App) runWrapped(acc account.Account, folder string, args []string) error {
	args = a.withFeedSettings(args)
	sess := a.beginSession(acc, folder)
	defer sess.finish()
	// before the start, so that a signal in between is held and not lost
	sigs := make(chan os.Signal, 8)
	// Notify with no signals at all would take every signal
	if watch := watchedSignals(); len(watch) > 0 {
		signal.Notify(sigs, watch...)
	}
	defer signal.Stop(sigs)

	cwd, _ := os.Getwd()
	reg := sessionRegistry()
	reg.now = a.clock
	in, _ := a.In.(*os.File)
	err := wrapper.Run(
		wrapper.Session{Handle: sess.handle, Account: acc.Name, Folder: folder, Cwd: cwd, Args: args},
		wrapper.Deps{
			Registry: reg, Feed: realFeed(), Accounts: moveAccounts{a},
			Launcher: runLauncher{a: a, handle: sess.handle}, Terminal: wrapper.NewTerminal(in, a.Out),
			Signals: sigs, PassInterrupt: !a.interruptReachesClaude(), Now: a.clock,
			Notice: func(line string) { fmt.Fprintln(a.Err, line) },
		})
	return claudeExit(err)
}
