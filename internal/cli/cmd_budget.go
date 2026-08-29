package cli

import (
	"errors"
	"fmt"
	"strconv"
	"text/tabwriter"

	"github.com/dhitalkamal/clalt/internal/tui"
)

// cmdBudget shows or sets per-account spend caps.
// Usage:
//
//	clalt budget                          show budgets vs today/week spend
//	clalt budget <name> --daily 20        set the daily cap (USD)
//	clalt budget <name> --weekly 100      set the weekly cap
//	clalt budget <name> --clear           remove both caps
func (a *App) cmdBudget(args []string) error {
	if len(args) == 0 {
		return a.showBudgets()
	}
	name := ""
	daily, weekly := -1.0, -1.0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--daily":
			v, err := nextFloat(args, &i)
			if err != nil {
				return err
			}
			daily = v
		case "--weekly":
			v, err := nextFloat(args, &i)
			if err != nil {
				return err
			}
			weekly = v
		case "--clear":
			daily, weekly = 0, 0
		default:
			name = args[i]
		}
	}
	if name == "" {
		return errors.New("usage: clalt budget <name> [--daily N] [--weekly N] [--clear]")
	}
	if err := a.Store.SetBudget(name, daily, weekly); err != nil {
		return err
	}
	a.printf("budget updated for %q\n", name)
	return a.showBudgets()
}

func nextFloat(args []string, i *int) (float64, error) {
	if *i+1 >= len(args) {
		return 0, fmt.Errorf("%s needs a number", args[*i])
	}
	*i++
	v, err := strconv.ParseFloat(args[*i], 64)
	if err != nil {
		return 0, fmt.Errorf("bad number %q", args[*i])
	}
	return v, nil
}

func (a *App) showBudgets() error {
	data, err := a.buildDashboardData(false)
	if err != nil {
		return err
	}
	if a.JSON {
		type row struct {
			Name    string  `json:"name"`
			Daily   float64 `json:"dailyBudget"`
			Weekly  float64 `json:"weeklyBudget"`
			Today   float64 `json:"todayCost"`
			Week    float64 `json:"weekCost"`
			OverDay bool    `json:"overDaily"`
		}
		var rows []row
		for _, v := range data.Accounts {
			rows = append(rows, row{v.Name, v.DailyBudget, v.WeeklyBudget,
				v.Stats.Today.Cost, weekCost(v), v.DailyBudget > 0 && v.Stats.Today.Cost >= v.DailyBudget})
		}
		return a.emitJSON(rows)
	}
	tw := tabwriter.NewWriter(a.Out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ACCOUNT\tTODAY\tDAILY CAP\tWEEK\tWEEKLY CAP")
	for _, v := range data.Accounts {
		fmt.Fprintf(tw, "%s\t$%.2f\t%s\t$%.2f\t%s\n",
			v.Name, v.Stats.Today.Cost, capLabel(v.DailyBudget), weekCost(v), capLabel(v.WeeklyBudget))
	}
	return tw.Flush()
}

func capLabel(v float64) string {
	if v <= 0 {
		return "-"
	}
	return fmt.Sprintf("$%.0f", v)
}

// weekCost sums the last 7 days of billable cost from an account's daily series.
func weekCost(v tui.AccountView) float64 {
	d := v.Daily
	if len(d) > 7 {
		d = d[len(d)-7:]
	}
	var sum float64
	for _, b := range d {
		sum += b.Cost
	}
	return sum
}
