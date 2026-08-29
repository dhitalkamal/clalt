package cli

import (
	"errors"
	"os/exec"

	"github.com/dhitalkamal/clalt/internal/tui"
)

func (a *App) cmdSwitch(args []string) error {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	if name == "" {
		accts, err := a.Store.List()
		if err != nil {
			return err
		}
		if len(accts) == 0 {
			return errors.New("no accounts to switch to; add one with 'clalt add' or 'clalt login'")
		}
		picked, err := tui.PickAccount(accts)
		if err != nil {
			return err
		}
		if picked == "" {
			a.printf("cancelled\n")
			return nil
		}
		name = picked
	}

	if err := a.Store.Switch(name); err != nil {
		return err
	}

	if a.JSON {
		return a.emitJSON(map[string]any{"active": name})
	}
	a.printf("switched to %q\n", name)
	if claudeIsRunning() {
		a.printf("note: a running 'claude' process keeps its current credential; restart it to pick up the switch.\n")
	}
	return nil
}

// claudeIsRunning best-effort checks for a live claude process so we can warn
// that an in-flight session will not see the switch until it restarts.
func claudeIsRunning() bool {
	if _, err := exec.LookPath("pgrep"); err != nil {
		return false
	}
	return exec.Command("pgrep", "-x", "claude").Run() == nil
}
