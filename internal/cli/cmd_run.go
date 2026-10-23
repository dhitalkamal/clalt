package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/claudecli"
	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/profile"
)

const runUsage = "clalt run <name> [-- claude args]"

// exitError ends the command with the exit code of the claude it ran, and
// prints nothing: what went wrong is claude's to say.
type exitError struct{ code int }

func (e exitError) Error() string { return "exit status " + strconv.Itoa(e.code) }

// cmdRun starts claude as an account. it makes sure the account's folder is
// linked to the shared claude home and is signed in as the person it was set up
// for, then gets out of the way.
func (a *App) cmdRun(args []string) error {
	label, claudeArgs, err := parseRunArgs(args)
	if err != nil {
		return err
	}
	acc, found, err := a.Store.ResolveLabel(label)
	switch {
	case err != nil:
		return err
	case !found:
		return fmt.Errorf("no account called %q. sign in with: clalt login %s", label, label)
	case acc.Meta.Hidden:
		return fmt.Errorf("%q was removed. 'clalt login %s' brings it back", label, label)
	}
	folder, err := folderFor(acc)
	if err != nil {
		return err
	}
	if err := a.refuseSharedFolder(acc); err != nil {
		return err
	}

	rep, err := a.syncAccount(folder)
	if err != nil {
		return err
	}
	if problem := projectsProblem(rep, folder); problem != "" {
		return errors.New("history would not be shared: " + problem)
	}
	a.noteStripped(folder)

	st, err := liveStatus(folder)
	if err != nil {
		return fmt.Errorf("could not check who %s is signed in as: %w", shown(acc), err)
	}
	if ok, why, text := checkSignIn(st, folder, acc.Meta); !ok {
		return fmt.Errorf("%s is not ready to run: %s. %s", shown(acc), text, loginAdvice(why, shown(acc)))
	}
	if err := a.Store.Touch(acc.Name); err != nil {
		fmt.Fprintf(a.Err, "warning: could not record the use of %s: %v\n", shown(acc), err)
	}
	return a.runClaude(acc, folder, claudeArgs)
}

// parseRunArgs takes the account name, which is the first argument, and hands
// everything else to claude, with or without a double dash in front of it.
func parseRunArgs(args []string) (label string, claudeArgs []string, err error) {
	for i, arg := range args {
		switch {
		case arg == "--":
			return finishRunArgs(label, append(claudeArgs, args[i+1:]...))
		case label != "":
			claudeArgs = append(claudeArgs, arg)
		case strings.HasPrefix(arg, "-"):
			return "", nil, usageError{"unknown flag " + arg, runUsage}
		default:
			label = arg
		}
	}
	return finishRunArgs(label, claudeArgs)
}

func finishRunArgs(label string, claudeArgs []string) (string, []string, error) {
	if label == "" {
		return "", nil, usageError{"which account? give it a name", runUsage}
	}
	return label, claudeArgs, nil
}

// projectsProblem says why the account's history would not be the shared
// history, or returns "" when it would be. it looks at the folder itself and not
// only at what the sync reported: claude makes a real projects folder when
// there is none, and then no other account could resume what was said here.
func projectsProblem(rep profile.Report, folder string) string {
	state := historyOf(folder)
	if state == historyShared {
		return ""
	}
	link := filepath.Join(folder, "projects")
	shared := filepath.Join(config.SharedClaudeDir(), "projects")
	for _, e := range rep.Errors {
		if e.Name == "projects" {
			return "projects could not be linked"
		}
	}
	if state == historyOwn {
		why := ""
		for _, st := range rep.Kept {
			if st.Name == "projects" {
				why = " (" + st.Why + ")"
			}
		}
		return fmt.Sprintf("%s is not linked to the shared history%s. move what is in it into %s, remove it and run again. 'clalt doctor' says more",
			link, why, shared)
	}
	return fmt.Sprintf("%s does not lead to the shared history folder %s. that folder is not reachable (an unmounted volume?), or the link aims somewhere else. 'clalt doctor' says more",
		link, shared)
}

// runClaude runs the session with the terminal handed over, and records it for
// `clalt sessions` while it lasts. clalt stays until claude is gone, whatever
// signal it gets in the meantime, see relaySignals.
func (a *App) runClaude(acc account.Account, folder string, args []string) error {
	cmd, err := claudecli.Session(context.Background(), folder, args)
	if err != nil {
		return err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = a.In, a.Out, a.Err
	// before the start, so that a signal in between is held and not lost
	sigs := make(chan os.Signal, 8)
	// Notify with no signals at all would take every signal
	if watch := watchedSignals(); len(watch) > 0 {
		signal.Notify(sigs, watch...)
	}
	defer signal.Stop(sigs)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start claude: %w", err)
	}
	over := make(chan struct{})
	go relaySignals(sigs, cmd.Process, over)
	pid := cmd.Process.Pid
	cwd, _ := os.Getwd()
	_ = profile.WriteSession(config.SessionsDir(), profile.Session{
		Account: acc.Name, PID: pid, Cwd: cwd, StartedAt: a.clock(),
	})
	err = cmd.Wait()
	close(over)
	_ = profile.RemoveSession(config.SessionsDir(), pid)
	return claudeExit(err)
}

// relaySignals deals with the signals that clalt gets while claude runs, until
// over is closed. a Ctrl-C reaches claude by itself, since both are in the
// terminal's foreground group, and what it means is claude's to decide, so it is
// only taken in: clalt has to be there when claude ends, and the shell must not
// get the terminal back before. it is caught and not ignored, because an ignored
// signal is inherited through exec and claude would never see its own Ctrl-C.
// SIGTERM and SIGHUP are for clalt alone (kill, an ide that stops it, a terminal
// that is closed), and claude would be left running with nobody to answer to, so
// they are passed on, and clalt ends with the code claude ends with.
func relaySignals(sigs <-chan os.Signal, claude *os.Process, over <-chan struct{}) {
	for {
		select {
		case s := <-sigs:
			if s != os.Interrupt {
				_ = claude.Signal(s)
			}
		case <-over:
			return
		}
	}
}

// watchedSignals are the signals that relaySignals deals with. one that was
// ignored when clalt started (nohup, a job that a script put in the background)
// is left alone: claude run by hand would inherit the ignore, and exec gives a
// caught signal back its default, so catching it would take the ignore away.
func watchedSignals() []os.Signal {
	var watch []os.Signal
	for _, s := range []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP} {
		if !signal.Ignored(s) {
			watch = append(watch, s)
		}
	}
	return watch
}

// claudeExit turns the way a session ended into what clalt exits with. a
// session killed by a signal gets the code a shell would report.
func claudeExit(err error) error {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return err
	}
	code := exit.ExitCode()
	if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		code = 128 + int(ws.Signal())
	}
	if code <= 0 {
		code = 1
	}
	return exitError{code}
}
