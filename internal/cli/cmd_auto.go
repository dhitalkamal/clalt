package cli

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/usage"
)

// cmdAuto watches the active account and rotates to the next healthy one before
// it hits its 5h limit or blows its daily budget. It runs until Ctrl-C.
// Usage: clalt auto [--interval 5m] [--threshold 0.85]
func (a *App) cmdAuto(args []string) error {
	interval := 5 * time.Minute
	threshold := account.DefaultThreshold
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--interval":
			if i+1 >= len(args) {
				return errors.New("--interval needs a duration, e.g. 5m")
			}
			i++
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("bad interval %q", args[i])
			}
			interval = d
		case "--threshold":
			if i+1 >= len(args) {
				return errors.New("--threshold needs a value 0..1")
			}
			i++
			t, err := strconv.ParseFloat(args[i], 64)
			if err != nil || t <= 0 || t > 1 {
				return fmt.Errorf("bad threshold %q", args[i])
			}
			threshold = t
		}
	}

	chain, _, err := a.Store.Fallback()
	if err != nil {
		return err
	}
	if len(chain) < 2 {
		return errors.New("need at least two accounts to auto-rotate")
	}

	a.printf("auto-rotation started: interval %s, threshold %.0f%%. ctrl-c to stop.\n", interval, threshold*100)
	a.printf("note: --live usage is rate-limited; keep the interval generous.\n")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	tick := time.NewTicker(interval)
	defer tick.Stop()

	a.autoCheck(threshold) // run once immediately
	for {
		select {
		case <-stop:
			a.printf("auto-rotation stopped.\n")
			return nil
		case <-tick.C:
			a.autoCheck(threshold)
		}
	}
}

// autoCheck rotates once if the active account is over its 5h threshold or daily
// budget, and a healthier account exists.
func (a *App) autoCheck(threshold float64) {
	active, _ := a.Store.Active()
	if active == "" {
		return
	}
	reason := ""
	if util := a.fiveHourUtil(active); util >= threshold {
		reason = fmt.Sprintf("5h at %.0f%%", util*100)
	} else if over, budget, cost := a.overBudget(active); over {
		reason = fmt.Sprintf("today $%.2f >= $%.0f cap", cost, budget)
	}
	if reason == "" {
		return
	}
	chain, thr, _ := a.Store.Fallback()
	target := a.pickHealthy(chain, active, thr)
	stamp := time.Now().Format("15:04")
	if target == "" || target == active {
		a.printf("[%s] %s would rotate (%s) but no healthy alternative\n", stamp, active, reason)
		return
	}
	if err := a.Store.Switch(target); err != nil {
		a.printf("[%s] rotate failed: %v\n", stamp, err)
		return
	}
	a.printf("[%s] rotated %s -> %s (%s)\n", stamp, active, target, reason)
}

// overBudget reports whether an account's today spend is at or over its daily cap.
func (a *App) overBudget(name string) (bool, float64, float64) {
	_, meta, err := a.Store.Get(name)
	if err != nil || meta.DailyBudget <= 0 {
		return false, 0, 0
	}
	cost := a.todayCost(name)
	return cost >= meta.DailyBudget, meta.DailyBudget, cost
}

// todayCost is an account's local today spend estimate.
func (a *App) todayCost(name string) float64 {
	byAccount, err := a.recordsByAccount()
	if err != nil {
		return 0
	}
	return usage.Aggregate(byAccount[name], time.Now()).Today.Cost
}
