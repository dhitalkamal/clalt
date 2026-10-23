package cli

import (
	"fmt"
	"strings"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/rules"
)

// ruleChecks look at the folder rules of the visible accounts that have any: a
// rule for a folder that is gone, or that is not an absolute path, matches
// nothing, and an account whose rules are all like that serves no folder at all.
// an account nobody set rules for gets no line. without a list of accounts there
// is nothing to look at, and the state check already says why.
func (a *App) ruleChecks() []checkResult {
	accts, err := a.visibleAccounts()
	if err != nil {
		return nil
	}
	all, err := a.Store.AllRules()
	if err != nil {
		return nil
	}
	var out []checkResult
	for _, acc := range accts {
		if r, ok := all[acc.Name]; ok {
			out = append(out, ruleCheck(acc, r))
		}
	}
	return out
}

func ruleCheck(acc account.Account, r account.Rules) checkResult {
	label := acc.Label()
	if bad := rules.Problems(r.Folders); len(bad) > 0 {
		parts := make([]string, len(bad))
		for i, p := range bad {
			parts[i] = fmt.Sprintf("%s (%s)", clean(p.Folder), p.Why)
		}
		return result(label, "rules", statusWarn,
			"%s can match nothing: %s. remove one with: clalt rules %s --remove-folder <folder>",
			count(len(bad), "rule"), strings.Join(parts, ", "), label)
	}
	folders := "no folder rules (any folder)"
	if n := len(r.Folders); n > 0 {
		folders = count(n, "folder")
	}
	return result(label, "rules", statusOK, "%s, auto-switch %s", folders, onOff(r.AutoSwitch))
}
