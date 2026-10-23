package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dhitalkamal/clalt/internal/crew"
)

// crewRow is a gateway as status prints it, and as --json gives it. there is
// no token in it, and there never is.
type crewRow struct {
	Account       string    `json:"account"`
	ID            string    `json:"id"`
	State         string    `json:"state"`
	Why           string    `json:"why,omitempty"`
	Port          int       `json:"port,omitempty"`
	URL           string    `json:"url,omitempty"`
	PID           int       `json:"pid,omitempty"`
	Version       string    `json:"version,omitempty"`
	StartedAt     time.Time `json:"startedAt,omitzero"`
	UptimeSeconds int64     `json:"uptimeSeconds,omitempty"`
	Home          string    `json:"home,omitempty"`
	Log           string    `json:"log,omitempty"`
}

func mustPaths(id string) crew.Paths {
	p, _ := crew.PathsFor(id)
	return p
}

func crewRowOf(label string, st crew.Status) crewRow {
	r := crewRow{Account: label, ID: st.Account, State: string(st.State), Why: clean(st.Why), Log: st.Paths.Log}
	if st.Record.PID > 0 {
		r.Port, r.PID, r.Version, r.StartedAt = st.Record.Port, st.Record.PID, clean(st.Record.Version), st.Record.StartedAt
		r.URL = "http://127.0.0.1:" + strconv.Itoa(r.Port)
		r.Home = st.Record.Home
		r.UptimeSeconds = int64(st.Uptime.Seconds())
	}
	return r
}

func (a *App) crewStatus(args []string) error {
	const usage = "clalt crew status [name] [--json]"
	var label string
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "-"):
			return usageError{"unknown flag " + arg, usage}
		case label != "":
			return usageError{"unexpected argument " + arg, usage}
		default:
			label = arg
		}
	}
	m := a.crewManager()
	labels := a.crewLabels()
	var rows []crewRow
	if label != "" {
		id, name, err := a.crewTarget(label)
		if err != nil {
			return err
		}
		st, err := m.Status(context.Background(), id)
		if err != nil {
			return err
		}
		rows = []crewRow{crewRowOf(name, st)}
	} else {
		all, err := m.StatusAll(context.Background())
		if err != nil {
			return err
		}
		for _, st := range all {
			rows = append(rows, crewRowOf(labelFor(labels, st.Account), st))
		}
	}
	if a.JSON {
		if rows == nil {
			rows = []crewRow{}
		}
		return a.emitJSON(rows)
	}
	if len(rows) == 0 {
		a.printf("no gateways. start one with 'clalt crew start <name>'.\n")
		return nil
	}
	tw := tabwriter.NewWriter(a.out(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ACCOUNT\tSTATE\tPORT\tPID\tVERSION\tUPTIME")
	for _, r := range rows {
		up := "-"
		if r.State == string(crew.StateRunning) {
			up = uptime(r.StartedAt)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Account, r.State, dashInt(r.Port), dashInt(r.PID), dash(r.Version), up)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if label != "" {
		r := rows[0]
		if r.URL != "" {
			a.printf("\nurl:  %s\nhome: %s\n", r.URL, r.Home)
		}
		if r.Why != "" {
			a.printf("\n%s\n", r.Why)
		}
		a.printf("log:  %s\n", r.Log)
	}
	return nil
}

func dashInt(n int) string {
	if n == 0 {
		return "-"
	}
	return strconv.Itoa(n)
}

func (a *App) crewStop(args []string) error {
	label, err := oneAccountArgs(args, "clalt crew stop <name>")
	if err != nil {
		return err
	}
	id, name, err := a.crewTarget(label)
	if err != nil {
		return err
	}
	res, err := a.crewManager().Stop(context.Background(), id)
	if err != nil {
		return err
	}
	switch res.Outcome {
	case crew.Stopped:
		a.printf("stopped the gateway for %s (pid %d)\n", name, res.Record.PID)
	case crew.Cleared:
		a.printf("the gateway for %s was not running (%s). removed its record, nothing was signalled\n", name, clean(res.Why))
	default:
		a.printf("no gateway of %s is recorded, nothing to stop\n", name)
	}
	return nil
}

func (a *App) crewLogs(args []string) error {
	const usage = "clalt crew logs <name> [-n lines]"
	lines, label := 100, ""
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "-n" || arg == "--lines":
			if i+1 >= len(args) {
				return usageError{arg + " needs a number", usage}
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n <= 0 {
				return usageError{"not a number of lines: " + args[i+1], usage}
			}
			lines = n
			i++
		case strings.HasPrefix(arg, "-"):
			return usageError{"unknown flag " + arg, usage}
		case label != "":
			return usageError{"unexpected argument " + arg, usage}
		default:
			label = arg
		}
	}
	if label == "" {
		return usageError{"which account? give it a name", usage}
	}
	id, name, err := a.crewTarget(label)
	if err != nil {
		return err
	}
	out, err := a.crewManager().Logs(id, lines)
	if errors.Is(err, crew.ErrNoLog) {
		return fmt.Errorf("%s: %w", name, err)
	}
	if err != nil {
		return err
	}
	for _, line := range out {
		a.printf("%s\n", clean(line))
	}
	return nil
}
