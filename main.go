// Command clalt manages multiple Claude accounts for Claude Code: capture,
// authenticate, switch, and inspect usage. See README.md for details.
package main

import (
	"os"

	"github.com/dhitalkamal/clalt/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
