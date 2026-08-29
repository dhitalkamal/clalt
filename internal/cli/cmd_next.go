package cli

import "errors"

// cmdNext rotates the active account along the fallback chain. Without --live it
// simply moves to the next account in the chain (run it when you hit a limit).
// With --live it skips accounts whose 5h utilization is at or above the
// threshold, picking the first healthy one.
func (a *App) cmdNext(args []string) error {
	live := false
	for _, arg := range args {
		if arg == "--live" {
			live = true
		}
	}

	chain, threshold, err := a.Store.Fallback()
	if err != nil {
		return err
	}
	if len(chain) < 2 {
		return errors.New("need at least two accounts to rotate; add another with 'clalt add' or 'clalt login'")
	}
	active, _ := a.Store.Active()

	var target string
	if live {
		target = a.pickHealthy(chain, active, threshold)
	} else {
		target = nextInChain(chain, active)
	}

	if target == "" || target == active {
		a.printf("staying on %q (no better account available)\n", active)
		return nil
	}
	if err := a.Store.Switch(target); err != nil {
		return err
	}
	if a.JSON {
		return a.emitJSON(map[string]any{"from": active, "to": target})
	}
	a.printf("rotated %s-> %q\n", fromLabel(active), target)
	if claudeIsRunning() {
		a.printf("note: restart any running 'claude' to pick up the new account.\n")
	}
	return nil
}

// pickHealthy returns the first chain account (other than active) whose live 5h
// utilization is below threshold; if none are below, the one with the lowest
// utilization; empty if there is no alternative.
func (a *App) pickHealthy(chain []string, active string, threshold float64) string {
	best, bestUtil := "", 2.0 // sentinel above any real 0..1 utilization
	for _, name := range chain {
		if name == active {
			continue
		}
		util := a.fiveHourUtil(name)
		if util < threshold {
			return name // first healthy account in chain order
		}
		if util < bestUtil {
			best, bestUtil = name, util
		}
	}
	return best
}

// fiveHourUtil returns an account's live 5h utilization (0 when unknown, so an
// account with no data is treated as available).
func (a *App) fiveHourUtil(name string) float64 {
	cred, _, err := a.Store.Get(name)
	if err != nil {
		return 0
	}
	lu, _ := a.liveUsage(name, cred)
	if lu == nil {
		return 0
	}
	return lu.FiveHour.Utilization
}

// nextInChain returns the account after active, wrapping around; if active is not
// in the chain it returns the first entry.
func nextInChain(chain []string, active string) string {
	idx := -1
	for i, n := range chain {
		if n == active {
			idx = i
			break
		}
	}
	return chain[(idx+1)%len(chain)]
}

func fromLabel(active string) string {
	if active == "" {
		return ""
	}
	return "from " + active + " "
}
