package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/notify"
)

const notifyUsage = "clalt notify [on|off|status|test] or clalt notify idle on|off"

// cmdNotify turns the needs-you notifications on or off, shows how they would
// show, or sends a sample.
func (a *App) cmdNotify(args []string) error {
	if len(args) == 2 && args[0] == "idle" && (args[1] == "on" || args[1] == "off") {
		if err := notify.SetIdleEnabled(filepath.Join(config.ClaltDir(), "notify"), args[1] == "on"); err != nil {
			return fmt.Errorf("save the notify setting: %w", err)
		}
		a.printf("idle prompt notifications are %s\n", args[1])
		return nil
	}
	word := "status"
	if len(args) > 0 {
		word = args[0]
	}
	if len(args) > 1 || (word != "on" && word != "off" && word != "status" && word != "test") {
		return usageError{"expected one of on, off, status, test", notifyUsage}
	}
	dir := filepath.Join(config.ClaltDir(), "notify")
	switch word {
	case "on", "off":
		if err := notify.SetEnabled(dir, word == "on"); err != nil {
			return fmt.Errorf("save the notify setting: %w", err)
		}
		a.printf("needs-you notifications are %s\n", word)
		return nil
	case "test":
		return a.notifyTest()
	}
	return a.notifyStatus(dir)
}

func (a *App) notifyStatus(dir string) error {
	on, readErr := notify.Enabled(dir)
	state := "off"
	if on {
		state = "on"
	}
	a.printf("needs-you notifications are %s\n", state)
	d := a.deps()
	_, ok := notify.System(d.runner, d.goos)
	switch {
	case !ok:
		a.printf("they would show: nothing available on this machine\n")
	case d.goos == "darwin":
		a.printf("they would show: macOS notification (osascript)\n")
	default:
		a.printf("they would show: notify-send\n")
	}
	if readErr != nil {
		a.printf("the settings file could not be read (%v), so the default (on) applies\n", readErr)
	}
	return nil
}

func (a *App) notifyTest() error {
	d := a.deps()
	n, ok := notify.System(d.runner, d.goos)
	if !ok {
		return errors.New("no notifier is available on this machine")
	}
	if err := notify.Test(context.Background(), n); err != nil {
		return fmt.Errorf("send the test notification: %w", err)
	}
	a.printf("sent a test notification\n")
	return nil
}
