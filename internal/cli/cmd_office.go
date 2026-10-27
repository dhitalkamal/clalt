package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/notify"
	"github.com/dhitalkamal/clalt/internal/office"
	"github.com/dhitalkamal/clalt/internal/office/layout"
	"github.com/dhitalkamal/clalt/internal/registry"
)

const officeUsage = "clalt office [--port N] [--no-open] [--template <id>]"

// officeArgs are the flags of clalt office.
type officeArgs struct {
	port   int
	noOpen bool
	// template is the starting template of this run, empty for the active one
	template string
}

func parseOfficeArgs(args []string) (officeArgs, error) {
	var out officeArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		val, hasVal := strings.CutPrefix(arg, "--port=")
		tmpl, hasTmpl := strings.CutPrefix(arg, "--template=")
		switch {
		case arg == "--template" || hasTmpl:
			if arg == "--template" {
				i++
				if i >= len(args) {
					return out, usageError{"--template needs a template id", officeUsage}
				}
				tmpl = args[i]
			}
			if !layout.IsTemplate(tmpl) {
				return out, usageError{"--template must be one of " + templateIDs(), officeUsage}
			}
			out.template = tmpl
		case arg == "--no-open":
			out.noOpen = true
		case arg == "--edit":
			// no longer needed, the page can always save; accepted so old scripts keep working
		case arg == "--port" || hasVal:
			if arg == "--port" {
				i++
				if i >= len(args) {
					return out, usageError{"--port needs a number", officeUsage}
				}
				val = args[i]
			}
			n, err := strconv.Atoi(val)
			if err != nil || n < 0 || n > 65535 {
				return out, usageError{"--port needs a number from 0 to 65535", officeUsage}
			}
			out.port = n
		case strings.HasPrefix(arg, "-"):
			return out, usageError{"unknown flag " + arg, officeUsage}
		default:
			return out, usageError{"unexpected argument " + arg, officeUsage}
		}
	}
	return out, nil
}

// templateIDs is the template list for a message: rows, village, ...
func templateIDs() string {
	ids := make([]string, len(layout.Templates))
	for i, t := range layout.Templates {
		ids[i] = t.ID
	}
	return strings.Join(ids, ", ")
}

func officeLogPath() string { return filepath.Join(config.LogsDir(), "office.log") }

// officeAccounts lists the accounts the dashboard lists, by label.
type officeAccounts struct{ app *App }

func (o officeAccounts) Accounts() ([]office.Account, error) {
	accts, err := o.app.visibleAccounts()
	if err != nil {
		return nil, err
	}
	out := make([]office.Account, 0, len(accts))
	for _, acc := range accts {
		out = append(out, office.Account{ID: acc.Name, Label: acc.Label()})
	}
	return out, nil
}

// officeReal is this machine's registry, feed, accounts and transcripts.
func (a *App) officeReal() office.Real {
	r := office.Real{
		Registry: registry.Registry{Dir: config.SessionsDir()},
		Feed:     feed.Store{Dir: config.FeedDir()},
		Accounts: officeAccounts{app: a},
		Team:     office.NewTeamFiles(userAgentsDir(), a.clock),
		Now:      a.clock,
	}
	if dir := config.ClaudeProjectsDir(); dir != "" {
		r.Projects = []string{dir}
	}
	return r
}

// officeSources is the office's view of this machine.
func (a *App) officeSources() office.Sources { return office.RealSources(a.officeReal()) }

// userAgentsDir is the agents folder of the claude home, found the way the
// account folders find it. empty when there is no home.
func userAgentsDir() string {
	home := config.SharedClaudeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, "agents")
}

// cmdOffice serves the live 3D office on loopback and opens it in the browser.
// it only reads: no account, session or file of claude is changed. the page can
// save office layouts, which are clalt's own files.
func (a *App) cmdOffice(args []string) error {
	if len(args) > 0 && args[0] == "layout" {
		return a.cmdOfficeLayout(args[1:])
	}
	opt, err := parseOfficeArgs(args)
	if err != nil {
		return err
	}
	ctx := a.officeCtx
	if ctx == nil {
		var stop context.CancelFunc
		ctx, stop = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
	}
	real := a.officeReal()
	src := office.RealSources(real)
	logf := officeLogf(dashboardLogger(officeLogPath()))
	revs := a.officeReviews(real, logf)
	src.Reviews = revs
	// the watcher lives as long as the office does and stops with it
	ctx, stopRevs := context.WithCancel(ctx)
	revsDone := make(chan struct{})
	go func() { defer close(revsDone); revs.Run(ctx) }()
	defer func() { stopRevs(); <-revsDone }()
	// every snapshot also feeds the garden generators their desk needs
	gen := office.NewGardenFeed(a.clock)
	warner := notify.Default()
	return cleanedOrNil(office.Run(ctx, office.Options{
		Port: opt.port, NoOpen: opt.noOpen, Open: a.officeOpen, Out: a.Out, Template: opt.template,
		Layouts:   layout.NewStore(config.OfficeDir(), officeLogf(dashboardLogger(officeLogPath()))),
		Generator: gen,
		Activity:  real,
		Reviews:   revs,
		Actions:   a.officeActions(),
		Summary:   a.officeSummary(),
		Snapshot: func() (office.Snapshot, error) {
			snap, err := office.Build(src)
			if err == nil {
				gen.Observe(snap)
				a.warnLimits(snap, warner)
			}
			return snap, err
		},
		Logf: officeLogf(dashboardLogger(officeLogPath())),
	}))
}

func officeLogf(log func(string)) func(string, ...any) {
	return func(format string, args ...any) { log(fmt.Sprintf(format, args...)) }
}

// warnLimits sends the 5h limit warnings of a snapshot in the background, so
// the page never waits for the desktop notification. once per window is the
// warner's own marker.
func (a *App) warnLimits(snap office.Snapshot, w office.LimitWarner) {
	soon := false
	for _, acc := range snap.Accounts {
		soon = soon || acc.Forecast.FiveHourAt != nil
	}
	if !soon {
		return
	}
	now := a.clock()
	logf := officeLogf(dashboardLogger(officeLogPath()))
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		office.WarnLimits(ctx, snap, now, w, logf)
	}()
}
