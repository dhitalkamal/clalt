package cli

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/dhitalkamal/clalt/internal/usage"
)

const budgetUsage = "clalt budget [name] [--daily N] [--weekly N] [--clear]"

// cmdBudget shows or sets per-account spend caps.
// Usage:
//
//	clalt budget                          show budgets vs today/week spend
//	clalt budget <name> --daily 20        set the daily cap (USD)
//	clalt budget <name> --weekly 100      set the weekly cap
//	clalt budget <name> --clear           remove both caps
func (a *App) cmdBudget(args []string) error {
	req, err := parseBudgetArgs(args)
	if err != nil {
		return err
	}
	if req.name == "" {
		return a.showBudgets()
	}
	acc, err := a.findVisible(req.name)
	if err != nil {
		return err
	}
	if !req.changes {
		return a.showBudgets()
	}
	if err := a.Store.SetBudget(acc.Name, req.daily, req.weekly); err != nil {
		return err
	}
	if !a.JSON {
		a.printf("budget updated for %s\n", shown(acc))
	}
	return a.showBudgets()
}

// budgetRequest is what the arguments of budget ask for: an account, which can be
// left out to see them all, and the caps to set on it, if any. -1 leaves a cap
// as it is and 0 clears it.
type budgetRequest struct {
	name          string
	daily, weekly float64
	changes       bool
}

// parseBudgetArgs reads the arguments. a cap is a number of dollars, 0 or more:
// this command writes the file that the budget bars are drawn from, so a mistyped
// flag, a second name or a number that is not one is an error and not a guess.
func parseBudgetArgs(args []string) (budgetRequest, error) {
	req := budgetRequest{daily: -1, weekly: -1}
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--daily" || arg == "--weekly":
			if i+1 >= len(args) {
				return req, usageError{arg + " needs a number of dollars", budgetUsage}
			}
			i++
			v, err := parseCap(arg, args[i])
			if err != nil {
				return req, err
			}
			if arg == "--daily" {
				req.daily = v
			} else {
				req.weekly = v
			}
			req.changes = true
		case arg == "--clear":
			req.daily, req.weekly, req.changes = 0, 0, true
		case strings.HasPrefix(arg, "-"):
			return req, usageError{"unknown flag " + arg, budgetUsage}
		case req.name != "":
			return req, usageError{"unexpected argument " + arg, budgetUsage}
		default:
			req.name = arg
		}
	}
	if req.changes && req.name == "" {
		return req, usageError{"which account? give it a name", budgetUsage}
	}
	return req, nil
}

func parseCap(flag, text string) (float64, error) {
	v, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0, usageError{fmt.Sprintf("bad number %q for %s: it is a number of dollars, 0 or more", text, flag), budgetUsage}
	}
	return v, nil
}

// budgetRow is one account's caps next to what it spent.
type budgetRow struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Daily   float64 `json:"dailyBudget"`
	Weekly  float64 `json:"weeklyBudget"`
	Today   float64 `json:"todayCost"`
	Week    float64 `json:"weekCost"`
	OverDay bool    `json:"overDaily"`
}

func (a *App) showBudgets() error {
	byAccount, err := a.recordsByAccount()
	if err != nil {
		return err
	}
	accts, err := a.visibleAccounts()
	if err != nil {
		return err
	}
	now := a.clock()
	rows := make([]budgetRow, 0, len(accts))
	for _, acc := range accts {
		recs := byAccount[acc.Name]
		row := budgetRow{
			ID: acc.Name, Name: shown(acc), Daily: acc.Meta.DailyBudget, Weekly: acc.Meta.WeeklyBudget,
			Today: usage.Aggregate(recs, now).Today.Cost,
		}
		for _, day := range usage.DailyTrend(recs, 7, now) {
			row.Week += day.Cost
		}
		row.OverDay = row.Daily > 0 && row.Today >= row.Daily
		rows = append(rows, row)
	}
	if a.JSON {
		return a.emitJSON(rows)
	}
	tw := tabwriter.NewWriter(a.out(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ACCOUNT\tTODAY\tDAILY CAP\tWEEK\tWEEKLY CAP")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t$%.2f\t%s\t$%.2f\t%s\n", r.Name, r.Today, capLabel(r.Daily), r.Week, capLabel(r.Weekly))
	}
	return tw.Flush()
}

func capLabel(v float64) string {
	if v <= 0 {
		return "-"
	}
	return fmt.Sprintf("$%.0f", v)
}
