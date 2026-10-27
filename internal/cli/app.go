// Package cli wires the clalt packages together and dispatches subcommands.
package cli

import (
	"bufio"
	"context"
	"io"
	"os"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/officed"
)

// App holds the constructed dependencies for one invocation.
type App struct {
	Store *account.Store

	// what resume, launch, jump and notify reach outside clalt with, nil means
	// the real ones, see session_deps.go
	sdeps *sessionDeps

	JSON bool
	Out  io.Writer
	In   io.Reader
	Err  io.Writer

	reader *bufio.Reader // shared line reader for prompts

	// the first error of a write to Out in the command that is running, see out
	writeErr error

	// now is the clock; tests set it, nil means time.Now.
	now func() time.Time

	// terminal says whether the input of a session is a terminal. tests set it,
	// nil means look at In, see inputIsTerminal.
	terminal func() bool

	// officeCtx ends clalt office, nil means Ctrl-C. officeOpen opens the
	// browser, nil means the real one. tests set both.
	officeCtx  context.Context
	officeOpen func(url string) error
	// officeMgr and officeAuto are the background office and its start at login,
	// nil means the real ones. tests set them with fakes.
	officeMgr  *officed.Manager
	officeAuto *officed.Autostart

	// foreground says whether clalt is in the foreground group of a terminal.
	// tests set it, nil means ask the controlling terminal.
	foreground func() bool
}

func (a *App) clock() time.Time {
	if a.now != nil {
		return a.now()
	}
	return time.Now()
}

// NewApp builds an App backed by on-disk state. it reads and writes nothing
// until a command needs to: accounts hold no secrets, so there is no vault to
// open, and claude keeps each login in the account's own folder.
func NewApp() *App {
	return &App{
		Store: account.New(config.StatePath(), config.SwitchLogPath()),
		Out:   os.Stdout,
		In:    os.Stdin,
		Err:   os.Stderr,
	}
}
