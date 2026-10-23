package cli

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/profile"
)

// sessionRow is a live `clalt run` session. account is the id the session was
// recorded under, label what the account is called now.
type sessionRow struct {
	Account   string    `json:"account"`
	Label     string    `json:"label"`
	PID       int       `json:"pid"`
	Cwd       string    `json:"cwd"`
	StartedAt time.Time `json:"startedAt"`
}

// labelsByID maps every account id, removed ones included, to its label.
func (a *App) labelsByID() (map[string]string, error) {
	all, err := a.Store.List()
	if err != nil {
		return nil, err
	}
	labels := make(map[string]string, len(all))
	for _, acc := range all {
		labels[acc.Name] = shown(acc)
	}
	return labels, nil
}

// cmdSessions lists the live `clalt run` sessions.
func (a *App) cmdSessions(args []string) error {
	if len(args) > 0 {
		return usageError{"sessions takes no arguments", "clalt sessions [--json]"}
	}
	sessions, err := profile.ListSessions(config.SessionsDir())
	if err != nil {
		return err
	}
	labels, err := a.labelsByID()
	if err != nil {
		return err
	}
	rows := make([]sessionRow, 0, len(sessions))
	for _, s := range sessions {
		label, known := labels[s.Account]
		if !known {
			label = clean(s.Account)
		}
		rows = append(rows, sessionRow{Account: s.Account, Label: label, PID: s.PID, Cwd: clean(s.Cwd), StartedAt: s.StartedAt})
	}
	if a.JSON {
		return a.emitJSON(rows)
	}
	if len(rows) == 0 {
		a.printf("no running sessions. start one with 'clalt run <name>'.\n")
		return nil
	}
	tw := tabwriter.NewWriter(a.out(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ACCOUNT\tPID\tUPTIME\tCWD")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", r.Label, r.PID, uptime(r.StartedAt), r.Cwd)
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
