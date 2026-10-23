package cli

import (
	"fmt"
	"os"
	"slices"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/rules"
)

// cmdRules shows and edits the rules of an account: the folders it may serve and
// whether auto-switch may use it. rules decide automatic choices (which account
// `clalt run` picks, where a session may be moved), so nothing here asks claude.
func (a *App) cmdRules(args []string) error {
	req, err := parseRulesArgs(args)
	if err != nil {
		return err
	}
	// every argument is checked before anything is looked up or written
	add := make([]string, len(req.add))
	for i, raw := range req.add {
		if add[i], err = rules.Normalize(raw); err != nil {
			return usageError{err.Error(), rulesUsage}
		}
	}
	if p, clash := sameInBoth(add, req.remove); clash {
		return usageError{"the same folder is given to --add-folder and --remove-folder: " + clean(p), rulesUsage}
	}
	acc, err := a.findVisible(req.name)
	if err != nil {
		return err
	}
	if !req.changes() {
		return a.showRules(acc)
	}

	var ch ruleChange
	err = a.Store.UpdateRules(acc.Name, func(r account.Rules) (account.Rules, error) {
		next, done, err := applyRules(r, add, req.remove, req.autoSwitch)
		ch = done
		return next, err
	})
	if err != nil {
		return err
	}
	a.noteRuleChange(acc, ch)
	if !a.JSON {
		a.printf("rules updated for %s\n", shown(acc))
	}
	return a.showRules(acc)
}

// ruleChange is what an edit did, for the notes that go with it.
type ruleChange struct {
	added      []string
	becameOpen bool // the last folder rule went, so any folder is allowed now
	autoOn     bool // auto-switch was off and is on now
}

// applyRules makes the edit. removals come first and take a folder however it is
// spelled, a link to it or its case included, or exactly as it is stored, which
// is the only way to name a rule that is not a usable path. a removal that finds
// no rule is an error, so a mistyped folder does not look as if it worked. a
// folder that is already a rule is not added twice.
func applyRules(r account.Rules, add, remove []string, auto *bool) (account.Rules, ruleChange, error) {
	var ch ruleChange
	hadFolders := len(r.Folders) > 0
	for _, raw := range remove {
		kept, err := dropFolder(r.Folders, raw)
		if err != nil {
			return r, ch, err
		}
		r.Folders = kept
	}
	for _, p := range add {
		if slices.ContainsFunc(r.Folders, func(f string) bool { return rules.Same(f, p) }) {
			continue
		}
		r.Folders = append(r.Folders, p)
		ch.added = append(ch.added, p)
	}
	if auto != nil {
		ch.autoOn = *auto && !r.AutoSwitch
		r.AutoSwitch = *auto
	}
	ch.becameOpen = hadFolders && len(r.Folders) == 0
	return r, ch, nil
}

// dropFolder takes the rules that raw names out of folders.
func dropFolder(folders []string, raw string) ([]string, error) {
	match := func(f string) bool { return f == raw }
	if !slices.Contains(folders, raw) {
		p, err := rules.Normalize(raw)
		if err != nil {
			return nil, usageError{err.Error(), rulesUsage}
		}
		match = func(f string) bool { return rules.Same(f, p) }
		if !slices.ContainsFunc(folders, match) {
			return nil, fmt.Errorf("no rule for %s", clean(p))
		}
	}
	return slices.DeleteFunc(slices.Clone(folders), match), nil
}

// sameInBoth finds a folder that one list adds and the other removes. a removal
// that is no usable path is skipped here: it can only name a stored rule, and an
// added folder is always a usable path.
func sameInBoth(added, removed []string) (string, bool) {
	for _, raw := range removed {
		p, err := rules.Normalize(raw)
		if err != nil {
			continue
		}
		for _, q := range added {
			if rules.Same(p, q) {
				return q, true
			}
		}
	}
	return "", false
}

// noteRuleChange says what the change needs saying. it goes to stderr: stdout
// is the listing, or the json.
func (a *App) noteRuleChange(acc account.Account, ch ruleChange) {
	for _, p := range ch.added {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			fmt.Fprintf(a.Err, "warning: %s does not exist. the rule is added anyway\n", clean(p))
		}
	}
	if ch.becameOpen {
		fmt.Fprintf(a.Err, "note: %s has no folder rules now, so it may serve any folder\n", shown(acc))
	}
	if ch.autoOn {
		fmt.Fprintf(a.Err, "note: auto-switch moves running sessions between accounts. use it only for accounts "+
			"you may use for this work: each account's terms apply (see docs/move.md)\n")
	}
}

// rulesView is the rules of one account as shown and as json. the lists are
// never null, so a script can loop over them.
type rulesView struct {
	ID         string        `json:"id"`
	Label      string        `json:"label"`
	Folders    []string      `json:"folders"`
	AnyFolder  bool          `json:"anyFolder"`
	AutoSwitch bool          `json:"autoSwitch"`
	Problems   []ruleProblem `json:"problems"`
}

type ruleProblem struct {
	Folder string `json:"folder"`
	Why    string `json:"why"`
}

func (a *App) showRules(acc account.Account) error {
	r, err := a.Store.RulesOf(acc.Name)
	if err != nil {
		return err
	}
	view := rulesView{
		ID: acc.Name, Label: shown(acc), Folders: append([]string{}, r.Folders...),
		AnyFolder: len(r.Folders) == 0, AutoSwitch: r.AutoSwitch, Problems: []ruleProblem{},
	}
	why := map[string]string{}
	for _, p := range rules.Problems(r.Folders) {
		view.Problems = append(view.Problems, ruleProblem{Folder: p.Folder, Why: p.Why})
		why[p.Folder] = p.Why
	}
	if a.JSON {
		return a.emitJSON(view)
	}
	a.printf("%s\n", view.Label)
	if view.AnyFolder {
		a.printf("  %-13s any (no rules)\n", "folders:")
	}
	for i, f := range view.Folders {
		head, mark := "", ""
		if i == 0 {
			head = "folders:"
		}
		if w, bad := why[f]; bad {
			mark = "  (" + w + ")"
		}
		a.printf("  %-13s %s%s\n", head, clean(f), mark)
	}
	a.printf("  %-13s %s\n", "auto-switch:", onOff(view.AutoSwitch))
	return nil
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}
