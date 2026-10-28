package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/dhitalkamal/clalt/internal/mcp"
	"github.com/dhitalkamal/clalt/internal/office"
)

const mcpUsage = "clalt mcp"

// cmdMcp serves the read-only MCP server on stdin and stdout until the client
// closes its end. it reads the same sources as the office, writes nothing and
// opens no port. stdout carries protocol lines only, logs go to stderr.
func (a *App) cmdMcp(args []string) error {
	if len(args) > 0 {
		return usageError{"mcp takes no arguments", mcpUsage}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	src := a.officeSources()
	sum := a.officeSummary()
	return mcp.Serve(ctx, a.In, a.out(), mcp.Backend{
		Snapshot: func() (office.Snapshot, error) { return office.Build(src) },
		Day:      sum.Day,
		Now:      a.clock,
		Logf:     func(format string, v ...any) { fmt.Fprintf(a.Err, "clalt mcp: "+format+"\n", v...) },
	})
}
