package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dhitalkamal/clalt/internal/crew"
)

// crewStart runs a gateway for an account. the account has to be ready for
// `clalt run` in every way that matters: it exists, was not removed, shares the
// history, and is signed in as the person it was set up for.
func (a *App) crewStart(args []string) error {
	label, err := oneAccountArgs(args, "clalt crew start <name>")
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
	st, err := liveStatus(folder)
	if err != nil {
		return fmt.Errorf("could not check who %s is signed in as: %w", shown(acc), err)
	}
	if ok, why, text := checkSignIn(st, folder, acc.Meta); !ok {
		return fmt.Errorf("%s is not ready for a gateway: %s. %s", shown(acc), text, loginAdvice(why, shown(acc)))
	}

	res, err := a.crewManager().Start(context.Background(), crew.StartRequest{Account: acc.Name, Folder: folder})
	var fail *crew.StartFailure
	if errors.As(err, &fail) {
		for _, line := range fail.Output {
			fmt.Fprintf(a.Err, "  gateway: %s\n", clean(line))
		}
	}
	if err != nil {
		return err
	}
	if len(res.Removed) > 0 {
		fmt.Fprintf(a.Err, "note: not passing these variables to the gateway: %s\n", strings.Join(res.Removed, ", "))
	}
	if res.Warning != "" {
		fmt.Fprintf(a.Err, "warning: %s\n", clean(res.Warning))
	}
	row := crewRowOf(shown(acc), crew.Status{Account: acc.Name, State: crew.StateRunning, Record: res.Record, Paths: mustPaths(acc.Name)})
	if a.JSON {
		return a.emitJSON(row)
	}
	if res.Already {
		a.printf("the gateway for %s is already running: %s (pid %d, kirocrew %s)\n", row.Account, row.URL, row.PID, row.Version)
		return nil
	}
	a.printf("started the gateway for %s: %s (pid %d, kirocrew %s)\n", row.Account, row.URL, row.PID, row.Version)
	return nil
}
