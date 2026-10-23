package crew

import (
	"context"
	"fmt"
	"time"
)

// StopOutcome says what Stop did.
type StopOutcome int

// NotRunning: the account has no record. Cleared: the record was of a gateway
// that is gone, and only the record was removed. Stopped: the gateway was asked
// to stop and ended.
const (
	NotRunning StopOutcome = iota
	Cleared
	Stopped
)

// StopResult is what Stop found and did.
type StopResult struct {
	Outcome StopOutcome
	Record  Record
	// Why is what was found, for a record that was cleared.
	Why string
}

// Stop ends the gateway clalt started for the account. it signals only the pid
// in the account's own record, and only after ps shows the same start time as
// at the launch and a command line that is still a gateway on the recorded
// port: never by name, never by port alone. a pid that is gone, or that
// another process has now, only loses the record. one that cannot be confirmed
// is left alone. there is no escalation to SIGKILL.
func (m *Manager) Stop(ctx context.Context, id string) (StopResult, error) {
	if _, err := PathsFor(id); err != nil {
		return StopResult{}, err
	}
	var res StopResult
	err := m.locked(func() (err error) {
		res, err = m.stop(ctx, id)
		return err
	})
	return res, err
}

func (m *Manager) stop(ctx context.Context, id string) (StopResult, error) {
	rec, found, err := ReadRecord(id)
	switch {
	case err != nil:
		return StopResult{}, fmt.Errorf("%w. check by hand that no gateway of this account is running, then remove that file", err)
	case !found:
		return StopResult{Outcome: NotRunning}, nil
	}
	switch state, why := m.standing(ctx, rec); state {
	case StateStale:
		return StopResult{Outcome: Cleared, Record: rec, Why: why}, RemoveRecord(id)
	case StateUnverified:
		return StopResult{}, fmt.Errorf("%s. clalt will not signal it. look at it with: ps -p %d", why, rec.PID)
	}
	if err := m.o.Signal(rec.PID); err != nil {
		return StopResult{}, fmt.Errorf("could not signal pid %d: %w", rec.PID, err)
	}
	deadline := time.Now().Add(m.o.StopWait)
	for {
		// gone and replaced both mean it is not the gateway any more
		if state, _ := m.standing(ctx, rec); state == StateStale {
			return StopResult{Outcome: Stopped, Record: rec}, RemoveRecord(id)
		}
		if !time.Now().Before(deadline) {
			return StopResult{}, fmt.Errorf("pid %d was asked to stop and is still running after %s. try again later", rec.PID, m.o.StopWait)
		}
		select {
		case <-ctx.Done():
			return StopResult{}, ctx.Err()
		case <-time.After(m.o.Poll):
		}
	}
}
