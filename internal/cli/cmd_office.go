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

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/office"
	"github.com/dhitalkamal/clalt/internal/office/layout"
	"github.com/dhitalkamal/clalt/internal/registry"
)

const officeUsage = "clalt office [--port N] [--no-open] [--edit] [--template <id>]"

// officeArgs are the flags of clalt office.
type officeArgs struct {
	port   int
	noOpen bool
	edit   bool
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
			out.edit = true
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

// officeSources is the office's view of this machine.
func (a *App) officeSources() office.Sources {
	return office.RealSources(office.Real{
		Registry: registry.Registry{Dir: config.SessionsDir()},
		Feed:     feed.Store{Dir: config.FeedDir()},
		Accounts: officeAccounts{app: a},
		Now:      a.clock,
	})
}

// cmdOffice serves the live 3D office on loopback and opens it in the browser.
// it only reads: no account, session or file of claude is changed. with --edit
// the page can also save office layouts, which are clalt's own files.
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
	src := a.officeSources()
	// every snapshot also feeds the garden generators their desk needs
	gen := office.NewGardenFeed(a.clock)
	return cleanedOrNil(office.Run(ctx, office.Options{
		Port: opt.port, NoOpen: opt.noOpen, Open: a.officeOpen, Out: a.Out, Edit: opt.edit, Template: opt.template,
		Layouts:   layout.NewStore(config.OfficeDir(), officeLogf(dashboardLogger(officeLogPath()))),
		Generator: gen,
		Snapshot: func() (office.Snapshot, error) {
			snap, err := office.Build(src)
			if err == nil {
				gen.Observe(snap)
			}
			return snap, err
		},
		Logf: officeLogf(dashboardLogger(officeLogPath())),
	}))
}

func officeLogf(log func(string)) func(string, ...any) {
	return func(format string, args ...any) { log(fmt.Sprintf(format, args...)) }
}
