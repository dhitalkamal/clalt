package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/claudecli"
	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/dashboard"
	"github.com/dhitalkamal/clalt/internal/dashboard/live"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
	"github.com/dhitalkamal/clalt/internal/usage"
)

// dashboardHost gives the live dashboard the rules of the commands: it cannot
// import cli, which wires it, so what it needs comes through here.
type dashboardHost struct{ app *App }

var _ live.Host = (*dashboardHost)(nil)

// CheckSignIn is the check of `clalt list` and `clalt run`: claude's answer for
// the folder, judged against the recorded email.
func (h *dashboardHost) CheckSignIn(acc account.Account) (live.SignIn, error) {
	folder, err := folderFor(acc)
	if err != nil {
		return live.SignIn{}, err
	}
	st, err := liveStatus(folder)
	if err != nil {
		return live.SignIn{}, err
	}
	ok, why, text := checkSignIn(st, folder, acc.Meta)
	if !ok && (why == claudecli.ReasonOtherAuthMethod || why == claudecli.ReasonOtherProvider) {
		text += ". " + loginAdvice(why, shown(acc))
	}
	return live.SignIn{OK: ok, Reason: why, Text: clean(text), Email: st.Email, Org: st.OrgName, Plan: st.SubscriptionType}, nil
}

// ClaudeVersion is the version of claude, with the reason when it is too old.
func (h *dashboardHost) ClaudeVersion() (string, error) {
	_, version, err := claudeVersion()
	if err != nil {
		return "", cleanErr(err)
	}
	if ok, err := claudeIsOldEnough(version); err != nil || !ok {
		return clean(version), fmt.Errorf("claude %s is older than the minimum %s, update claude", clean(version), claudecli.MinVersion)
	}
	return clean(version), nil
}

// LinksInSync is the read-only link check of `clalt doctor`. a check that
// cannot be made is an error, not an answer.
func (h *dashboardHost) LinksInSync(folder string) (bool, error) {
	steps, _, err := planLinks(folder)
	if err != nil {
		return false, cleanErr(err)
	}
	return len(steps) == 0, nil
}

// Usage is the attribution of `clalt usage`.
func (h *dashboardHost) Usage() (map[string][]usage.Record, error) {
	return h.app.recordsByAccount()
}

// SignOut is the sign-out of `clalt logout`, with nothing printed: the
// dashboard owns the screen.
func (h *dashboardHost) SignOut(acc account.Account) error {
	return cleanedOrNil(h.app.signOut(acc))
}

// Delete is `clalt remove --delete --force` with nothing printed: the
// dashboard asked already, and owns the screen.
func (h *dashboardHost) Delete(acc account.Account) error {
	folder, err := h.app.checkDelete(acc)
	if err != nil {
		return cleanedOrNil(err)
	}
	return cleanedOrNil(h.app.performDelete(acc, folder))
}

// Run is the code of `clalt run <label>`.
func (h *dashboardHost) Run(acc account.Account) dashboard.Launch {
	return &dashLaunch{app: h.app, verb: "run", label: acc.Label(), call: h.app.cmdRun}
}

// Login is the code of `clalt login <label>`.
func (h *dashboardHost) Login(acc account.Account) dashboard.Launch {
	return &dashLaunch{app: h.app, verb: "login", label: acc.Label(), call: h.app.cmdLogin}
}

// dashLaunch runs a command of clalt while the dashboard has let go of the
// terminal. a refusal is printed and kept on screen until enter, since the
// dashboard takes the screen back at once, and the reason would be gone.
type dashLaunch struct {
	app   *App
	verb  string
	label string
	call  func(args []string) error
}

// the streams are the app's own, which are the terminal's
func (l *dashLaunch) SetStdin(io.Reader)  {}
func (l *dashLaunch) SetStdout(io.Writer) {}
func (l *dashLaunch) SetStderr(io.Writer) {}

// launchStatus is how claude ended, as the dashboard reads it
type launchStatus struct{ code int }

func (s launchStatus) Error() string { return fmt.Sprintf("exit status %d", s.code) }
func (s launchStatus) ExitCode() int { return s.code }

func (l *dashLaunch) Run() error {
	err := l.call([]string{l.label})
	l.app.afterDashAction(err, l.verb)
	var exit exitError
	if errors.As(err, &exit) {
		return launchStatus{exit.code}
	}
	return err
}

// maxDashLog is how big the dashboard log gets before it is set aside
const maxDashLog = 512 << 10

// dashboardLogger appends one cleaned line per failure to path, set aside as
// path.old once it passes maxDashLog. it never fails the dashboard: a log that
// cannot be written is just not there.
func dashboardLogger(path string) func(string) {
	return func(msg string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return
		}
		if fi, err := os.Stat(path); err == nil && fi.Size() > maxDashLog {
			_ = os.Rename(path, path+".old")
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return
		}
		defer f.Close()
		line := strings.NewReplacer("\r\n", " | ", "\n", " | ", "\r", " | ").Replace(msg)
		fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), clean(line))
	}
}

func dashboardLogPath() string { return filepath.Join(config.ClaltDir(), "logs", "dashboard.log") }

// liveAdapter is the dashboard's source and actions on this machine.
func (a *App) liveAdapter() *live.Adapter {
	home := config.Home()
	fd := feed.Store{Dir: config.FeedDir()}
	return live.New(live.Config{
		Store: a.Store, Host: &dashboardHost{app: a}, Now: a.clock, Home: home,
		Feed:     live.FeedFiles{Store: fd, Now: a.clock},
		Sessions: &live.RegistrySessions{Reg: registry.Registry{Dir: config.SessionsDir()}, Feed: fd, Now: a.clock},
		Gateways: live.CrewGateways{},
	})
}

// cleanedOrNil cleans an error that the dashboard will draw
func cleanedOrNil(err error) error {
	if err == nil {
		return nil
	}
	return cleanErr(err)
}
