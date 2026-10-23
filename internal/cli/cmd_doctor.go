package cli

import (
	"fmt"

	"github.com/dhitalkamal/clalt/internal/config"
)

const (
	statusOK   = "ok"
	statusWarn = "warn"
	statusFail = "fail"
)

// checkResult is one line of the report. scope says what was looked at: claude,
// an account label, the shell, the shared settings or a leftover of the old
// design. check names what was looked at there and stays the same between
// releases, for scripts.
type checkResult struct {
	Scope  string `json:"scope"`
	Check  string `json:"check"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type doctorReport struct {
	Checks []checkResult `json:"checks"`
	Failed int           `json:"failed"`
	Warned int           `json:"warned"`
}

// result builds a line of the report. the scope is often an account label from
// state.json, so it is cleaned like the detail.
func result(scope, check, status, format string, args ...any) checkResult {
	return checkResult{Scope: clean(scope), Check: check, Status: status, Detail: clean(fmt.Sprintf(format, args...))}
}

// cmdDoctor reports on claude, on every account and on what is left of the old
// design. it only looks: nothing is repaired or deleted, `clalt run` repairs
// links, and old files are for the user to remove.
func (a *App) cmdDoctor(args []string) error {
	if len(args) > 0 {
		return usageError{"doctor takes no arguments", "clalt doctor [--json]"}
	}
	rep := a.runDoctor()
	if a.JSON {
		if err := a.emitJSON(rep); err != nil {
			return err
		}
	} else {
		a.renderDoctor(rep)
	}
	if rep.Failed > 0 {
		return exitError{1}
	}
	return nil
}

func (a *App) runDoctor() doctorReport {
	claude, usable := claudeCheck()
	checks := []checkResult{claude}
	accounts, err := a.accountChecks(usable)
	if err != nil {
		// no list of accounts means nothing to check per account. the rest does
		// not need it, so it still runs
		checks = append(checks, result("state", "file", statusFail, "the account list at %s could not be read: %v", config.StatePath(), err))
	} else {
		checks = append(checks, stateOK())
	}
	checks = append(checks, accounts...)
	checks = append(checks, a.ruleChecks()...)
	checks = append(checks, envCheck(), settingsCheck())
	checks = append(checks, a.feedChecks()...)
	checks = append(checks, a.leftoverChecks()...)
	checks = append(checks, a.crewChecks()...)

	rep := doctorReport{Checks: checks}
	for _, c := range checks {
		switch c.Status {
		case statusFail:
			rep.Failed++
		case statusWarn:
			rep.Warned++
		}
	}
	return rep
}

// stateOK is the line for an account list that was read, or that is not there
// yet, which is how a machine looks before its first sign-in.
func stateOK() checkResult {
	path := config.StatePath()
	if there, err := folderThere(path); err == nil && !there {
		return result("state", "file", statusOK, "no account list yet, the first sign-in makes %s", path)
	}
	return result("state", "file", statusOK, "the account list at %s can be read", path)
}

func (a *App) renderDoctor(rep doctorReport) {
	for _, c := range rep.Checks {
		a.printf("%-5s %s: %s\n", c.Status, c.Scope, c.Detail)
	}
	if rep.Failed == 0 && rep.Warned == 0 {
		a.printf("\nall checks passed\n")
		return
	}
	a.printf("\n%s failed, %s\n", count(rep.Failed, "check"), count(rep.Warned, "warning"))
}
