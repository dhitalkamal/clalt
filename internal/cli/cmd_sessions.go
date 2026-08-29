package cli

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/profile"
)

// cmdSessions lists live `clalt run` sessions (parallel isolated profiles).
func (a *App) cmdSessions(args []string) error {
	sessions, err := profile.ListSessions(config.SessionsDir())
	if err != nil {
		return err
	}
	if a.JSON {
		return a.emitJSON(sessions)
	}
	if len(sessions) == 0 {
		a.printf("no running sessions. start one with 'clalt run <name>'.\n")
		return nil
	}
	tw := tabwriter.NewWriter(a.Out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ACCOUNT\tPID\tUPTIME\tCWD")
	for _, s := range sessions {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", s.Account, s.PID, uptime(s.StartedAt), s.Cwd)
	}
	return tw.Flush()
}

func uptime(start time.Time) string {
	if start.IsZero() {
		return "-"
	}
	d := time.Since(start).Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
