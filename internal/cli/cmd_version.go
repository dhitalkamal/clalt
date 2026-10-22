package cli

import "github.com/dhitalkamal/clalt/internal/version"

// cmdVersion prints build version info: a human-readable line by default, or
// the same data as JSON with --json (works with either the global --json flag
// or a trailing --json arg, since Run strips the global flag before dispatch).
func (a *App) cmdVersion(args []string) error {
	jsonOut := a.JSON
	for _, arg := range args {
		if arg == "--json" {
			jsonOut = true
		}
	}
	if jsonOut {
		return a.emitJSON(version.Info())
	}
	a.printf("%s\n", version.String())
	return nil
}
