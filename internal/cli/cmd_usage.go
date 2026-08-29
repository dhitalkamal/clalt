package cli

import (
	"fmt"
	"strings"

	"github.com/dhitalkamal/clalt/internal/usage"
)

func (a *App) cmdUsage(args []string) error {
	live, filter := parseUsageArgs(args)
	reports, err := a.buildReports(live, filter)
	if err != nil {
		return err
	}
	if a.JSON {
		return a.emitJSON(reports)
	}
	if len(reports) == 0 {
		a.printf("no accounts to report on\n")
		return nil
	}
	if live {
		a.printf("note: --live queries Anthropic's usage endpoint with your token (ToS gray area; see README).\n\n")
	}
	for i, r := range reports {
		if i > 0 {
			a.printf("\n")
		}
		a.renderReport(r)
	}
	a.printf("\nlocal stats are per-machine and attributed by switch history; use --live for authoritative windows.\n")
	return nil
}

// parseUsageArgs pulls the --live flag and an optional account name from args.
func parseUsageArgs(args []string) (live bool, filter string) {
	for _, arg := range args {
		switch {
		case arg == "--live":
			live = true
		case strings.HasPrefix(arg, "-"):
			// ignore unknown flags
		default:
			filter = arg
		}
	}
	return live, filter
}

func (a *App) renderReport(r accountReport) {
	marker := ""
	if r.Active {
		marker = " (active)"
	}
	a.printf("== %s%s ==\n", r.Name, marker)
	if r.Email != "" || r.Plan != "" {
		a.printf("  %s  %s\n", dash(r.Plan), dash(r.Email))
	}
	a.printf("  token:    %s\n", humanUntil(r.ExpiresAt))
	a.printf("  5h local: %s  %s\n", bar(windowFraction(r.FiveHour.Tokens)), tokensLabel(r.FiveHour.Tokens, r.FiveHour.Messages))
	a.printf("  7d local: %s  %s\n", bar(windowFraction(r.SevenDay.Tokens)), tokensLabel(r.SevenDay.Tokens, r.SevenDay.Messages))
	a.printf("  total:    %s tokens, $%.2f est\n", humanTokens(r.Tokens), r.Cost)

	if r.Live != nil {
		a.printf("  live 5h:  %s  %.0f%% used, resets %s\n", bar(r.Live.FiveHour.Utilization), r.Live.FiveHour.Utilization*100, humanUntil(r.Live.FiveHour.ResetsAt))
		a.printf("  live 7d:  %s  %.0f%% used, resets %s\n", bar(r.Live.SevenDay.Utilization), r.Live.SevenDay.Utilization*100, humanUntil(r.Live.SevenDay.ResetsAt))
	}
	if r.LiveNote != "" {
		a.printf("  live:     %s\n", r.LiveNote)
	}
	if len(r.Models) > 0 {
		a.printf("  by model:\n")
		for _, m := range r.Models {
			a.printf("    %-22s %10s tokens  $%.2f\n", dash(m.Model), humanTokens(m.Tokens), m.Cost)
		}
	}
	if len(r.Projects) > 0 {
		a.printf("  top projects:\n")
		for _, p := range r.Projects {
			a.printf("    %-30s %10s tokens\n", truncate(p.Project, 30), humanTokens(p.Tokens))
		}
	}
	if hasTrend(r.Trend) {
		a.printf("  14d trend: %s\n", sparkline(r.Trend))
	}
}

// windowFraction maps a local token count to a 0..1 bar. There is no local hard
// limit, so we scale against a nominal reference just for a visual sense.
func windowFraction(tokens int64) float64 {
	const ref = 2_000_000.0 // nominal reference for the bar only
	f := float64(tokens) / ref
	if f > 1 {
		f = 1
	}
	return f
}

// bar renders an ASCII progress bar for a 0..1 fraction.
func bar(frac float64) string {
	const width = 20
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac*float64(width) + 0.5)
	return "[" + strings.Repeat("#", filled) + strings.Repeat(".", width-filled) + "]"
}

// sparkline renders daily tokens as an ASCII bar row scaled to the max day.
func sparkline(buckets []usage.DayBucket) string {
	levels := []byte{' ', '.', ':', '-', '=', '+', '*', '#'}
	var max int64
	for _, b := range buckets {
		if b.Tokens > max {
			max = b.Tokens
		}
	}
	if max == 0 {
		return "(no usage)"
	}
	var sb strings.Builder
	for _, b := range buckets {
		idx := int(float64(b.Tokens) / float64(max) * float64(len(levels)-1))
		sb.WriteByte(levels[idx])
	}
	return sb.String()
}

func hasTrend(buckets []usage.DayBucket) bool {
	for _, b := range buckets {
		if b.Tokens > 0 {
			return true
		}
	}
	return false
}

func tokensLabel(tokens int64, messages int) string {
	return fmt.Sprintf("%s tokens over %d msgs", humanTokens(tokens), messages)
}

func humanTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "~"
}
