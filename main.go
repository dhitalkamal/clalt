// Command clalt manages several Claude accounts for Claude Code: it signs each
// one in through claude's own login, runs sessions for any of them side by side
// with one shared history, and reports their usage. See README.md for details.
package main

import (
	"os"

	"github.com/dhitalkamal/clalt/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
