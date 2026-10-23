// Package cli wires the clalt packages together and dispatches subcommands.
package cli

import (
	"bufio"
	"io"
	"os"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/config"
)

// App holds the constructed dependencies for one invocation.
type App struct {
	Store *account.Store

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
