package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhitalkamal/clalt/internal/terminal"
)

const launchUsage = "clalt launch <name> [--cwd dir] [-- claude args]"

// cmdLaunch opens `clalt run <name>` in a new tab of the terminal that is
// already there (tmux or kitty). before the tab opens it checks what run would,
// because a tab whose command fails closes with it and the error is lost.
func (a *App) cmdLaunch(args []string) error {
	label, cwd, claudeArgs, err := parseLaunchArgs(args)
	if err != nil {
		return err
	}
	acc, err := a.findVisible(label)
	if err != nil {
		return err
	}
	if cwd, err = launchDir(cwd); err != nil {
		return err
	}
	folder, err := folderFor(acc)
	if err != nil {
		return err
	}
	st, err := liveStatus(folder)
	if err != nil {
		return fmt.Errorf("could not check who %s is signed in as: %w", shown(acc), err)
	}
	if ok, why, text := checkSignIn(st, folder, acc.Meta); !ok {
		return fmt.Errorf("%s is not ready to run: %s. %s", shown(acc), text, loginAdvice(why, shown(acc)))
	}

	d := a.deps()
	exe, err := d.exe()
	if err != nil {
		return fmt.Errorf("could not find the clalt program: %w", err)
	}
	drv := terminal.Driver{Run: d.runner, Env: d.env}
	where, err := drv.Launch(context.Background(), terminal.LaunchSpec{Exe: exe, Account: acc.Label(), Cwd: cwd, Args: claudeArgs})
	if errors.Is(err, terminal.ErrNoTerminal) {
		plain := append([]string{"clalt", "run", acc.Label()}, withDash(claudeArgs)...)
		a.printf("%s. run this in the terminal you want:\ncd %s && %s\n", err, terminal.ShellLine([]string{cwd}), terminal.ShellLine(plain))
		return nil
	}
	if err != nil {
		return cleanErr(err)
	}
	a.printf("opened %s in %s\n", shown(acc), clean(where))
	return nil
}

func withDash(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	return append([]string{"--"}, args...)
}

// parseLaunchArgs reads the account and --cwd before a double dash. what follows
// the double dash is for claude.
func parseLaunchArgs(args []string) (label, cwd string, claudeArgs []string, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			claudeArgs = args[i+1:]
			i = len(args)
		case arg == "--cwd":
			if i+1 >= len(args) {
				return "", "", nil, usageError{"--cwd needs a folder", launchUsage}
			}
			i++
			cwd = args[i]
		case strings.HasPrefix(arg, "--cwd="):
			cwd = strings.TrimPrefix(arg, "--cwd=")
		case strings.HasPrefix(arg, "-"):
			return "", "", nil, usageError{"unknown flag " + arg, launchUsage}
		case label != "":
			return "", "", nil, usageError{"unexpected argument " + arg, launchUsage}
		default:
			label = arg
		}
	}
	if label == "" {
		return "", "", nil, usageError{"which account? give it a name", launchUsage}
	}
	return label, cwd, claudeArgs, nil
}

// launchDir is the absolute folder the tab opens in: the one asked for, or the
// current one. it has to exist.
func launchDir(cwd string) (string, error) {
	if cwd == "" {
		return os.Getwd()
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("--cwd %s: %w", clean(cwd), err)
	}
	info, err := os.Stat(abs)
	switch {
	case err != nil:
		return "", fmt.Errorf("--cwd %s: %s", clean(cwd), cleanErr(err))
	case !info.IsDir():
		return "", fmt.Errorf("--cwd %s is not a folder", clean(cwd))
	}
	return abs, nil
}
