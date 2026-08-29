package cli

import (
	"fmt"
	"strconv"
)

// cmdFallback shows or configures the rotation chain used by `clalt next`.
// Usage:
//
//	clalt fallback                       show the chain + threshold
//	clalt fallback <a> <b> <c>           set the chain order
//	clalt fallback --threshold 0.85      set the rotation threshold
//	clalt fallback clear                 revert to all-accounts order
func (a *App) cmdFallback(args []string) error {
	var names []string
	setThreshold := false
	var threshold float64

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "clear":
			return a.showFallback(a.Store.ClearFallback())
		case "--threshold":
			if i+1 >= len(args) {
				return fmt.Errorf("--threshold needs a value between 0 and 1")
			}
			t, err := strconv.ParseFloat(args[i+1], 64)
			if err != nil {
				return fmt.Errorf("bad threshold %q", args[i+1])
			}
			threshold, setThreshold = t, true
			i++
		default:
			names = append(names, args[i])
		}
	}

	if len(names) > 0 {
		if err := a.Store.SetFallback(names); err != nil {
			return err
		}
	}
	if setThreshold {
		if err := a.Store.SetThreshold(threshold); err != nil {
			return err
		}
	}
	return a.showFallback(nil)
}

func (a *App) showFallback(err error) error {
	if err != nil {
		return err
	}
	chain, threshold, err := a.Store.Fallback()
	if err != nil {
		return err
	}
	active, _ := a.Store.Active()
	if a.JSON {
		return a.emitJSON(map[string]any{"chain": chain, "threshold": threshold, "active": active})
	}
	a.printf("fallback chain (threshold %.0f%%):\n", threshold*100)
	for i, name := range chain {
		marker := " "
		if name == active {
			marker = "*"
		}
		a.printf("  %d. %s %s\n", i+1, marker, name)
	}
	a.printf("\nrun 'clalt next' to rotate to the next account, or 'clalt next --live' to skip accounts near their limit.\n")
	return nil
}
