package cli

import (
	"context"
	"path/filepath"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/notify"
	"github.com/dhitalkamal/clalt/internal/registry"
)

// notifyTimeout bounds the notification program inside a hook. claude gives the
// hook 5 seconds. the program is run in the hook and waited for, with this
// limit, instead of as a detached child: osascript and notify-send return in
// well under a second, a child that outlives the hook is an orphan nobody can
// time out or test, and a hung one costs the hook only this long.
var notifyTimeout = 1500 * time.Millisecond

// hookBudget is how long a whole hook may take, from its start. claude cuts a
// hook off at 5 seconds, and the notification must not be what gets it cut.
var hookBudget = 4 * time.Second

// minNotifyBudget is the least time left that is worth starting a notification
// for.
const minNotifyBudget = 200 * time.Millisecond

// capWait is wait, or what is left until deadline when that is less. never
// zero, which a lock would take as its default.
func capWait(wait time.Duration, deadline time.Time) time.Duration {
	if left := time.Until(deadline); left < wait {
		wait = left
	}
	if wait < time.Millisecond {
		wait = time.Millisecond
	}
	return wait
}

// needsYouReasons say what a session waits for, by notification type. claude's
// own message is not used, so no tool input can reach the screen.
var needsYouReasons = map[string]string{
	"permission_prompt":      "needs your permission",
	"elicitation_dialog":     "has a question",
	"elicitation_url_dialog": "has a question",
	"agent_needs_input":      "needs your input",
}

// hookNotify runs the notification side of one hook event, after the feed was
// updated. it ends the wait of the session on every event but the
// notifications, so the next wait may notify. needs says whether the event put
// the session into needs-you. it never prints and only logs failures.
func (a *App) hookNotify(rec registry.Record, event string, in feed.HookInput, needs bool, now time.Time, deadline time.Time) {
	if event != "Notification" {
		// only the folder is needed, not a notifier
		if err := a.alertsIn().EndWait(rec.Handle); err != nil {
			hookLog(now, event, "%s: end the notification wait: %v", rec.Handle, err)
		}
		return
	}
	alerts := a.alerts()
	reason, ok := needsYouReasons[in.NotificationType], needs
	if in.NotificationType == "idle_prompt" {
		on, _ := notify.IdleEnabled(alerts.Dir)
		reason, ok = "is waiting for your next prompt", on
	}
	if !ok {
		return
	}
	cwd := in.Cwd
	if cwd == "" {
		cwd = rec.Cwd
	}
	budget := min(notifyTimeout, time.Until(deadline))
	if budget < minNotifyBudget {
		hookLog(now, event, "%s: notify: no time left in the hook", rec.Handle)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	_, err := alerts.NeedsYou(ctx, notify.Event{Handle: rec.Handle, Account: a.accountLabel(rec.Account), Reason: reason, Cwd: cwd})
	if err != nil {
		hookLog(now, event, "%s: notify: %v", rec.Handle, err)
	}
}

// alertsIn is an Alerts that only knows its folder.
func (a *App) alertsIn() *notify.Alerts {
	return &notify.Alerts{Dir: filepath.Join(config.ClaltDir(), "notify"), Now: a.clock}
}

// alerts is the Alerts of this machine, with the runner of the deps.
func (a *App) alerts() *notify.Alerts {
	d := a.deps()
	n, _ := notify.System(d.runner, d.goos)
	return &notify.Alerts{Notifier: n, Dir: filepath.Join(config.ClaltDir(), "notify"), Now: a.clock}
}

// accountLabel is what the user calls account id, or id when it is not known.
func (a *App) accountLabel(id string) string {
	if a.Store == nil {
		return id
	}
	if accts, err := a.Store.List(); err == nil {
		for _, acc := range accts {
			if acc.Name == id {
				return acc.Label()
			}
		}
	}
	return id
}
