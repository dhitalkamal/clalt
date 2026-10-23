package cli

import (
	"context"

	"github.com/dhitalkamal/clalt/internal/crew"
)

// crewChecks report on the gateways clalt started and on the installed Kiro
// Crew. they say nothing until a gateway has a record, so people who do not
// use crew see no new lines and doctor does not run kirocrew for them. they
// only look: a stale record is for `clalt crew stop` to clear.
func (a *App) crewChecks() []checkResult {
	m := a.crewManager()
	all, err := m.StatusAll(context.Background())
	if err != nil {
		return []checkResult{result("crew", "crewGateways", statusWarn, "the gateway records could not be read: %v", err)}
	}
	if len(all) == 0 {
		return nil
	}
	labels := a.crewLabels()
	var out []checkResult
	for _, st := range all {
		label := labelFor(labels, st.Account)
		switch st.State {
		case crew.StateRunning:
			out = append(out, result(label, "crewGateway", statusOK, "gateway running on port %d (pid %d, kirocrew %s)", st.Record.Port, st.Record.PID, st.Record.Version))
		case crew.StateStale:
			out = append(out, result(label, "crewGateway", statusWarn, "%s. the gateway ended without 'clalt crew stop'. clear the record with: clalt crew stop %s", st.Why, label))
		default:
			out = append(out, result(label, "crewGateway", statusWarn, "gateway state %s: %s", st.State, st.Why))
		}
	}
	return append(out, a.crewVersionCheck(m))
}

func (a *App) crewVersionCheck(m *crew.Manager) checkResult {
	v, err := m.InstalledVersion(context.Background())
	if err != nil {
		return result("crew", "crewVersion", statusWarn, "%v", err)
	}
	if err := m.Accept(v); err != nil {
		return result("crew", "crewVersion", statusWarn, "%v. 'clalt crew start' will refuse it", err)
	}
	return result("crew", "crewVersion", statusOK, "kirocrew %s is a supported version", v)
}
