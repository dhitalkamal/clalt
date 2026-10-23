package crew

import (
	"context"
	"time"
)

// Status is one gateway as status shows it.
type Status struct {
	Account string
	State   State
	// Why says what was found for a gateway that is not running.
	Why    string
	Record Record
	// Uptime is how long a running gateway has been up.
	Uptime time.Duration
	Paths  Paths
}

// Status looks at the gateway of one account. it only looks: a stale record is
// reported and left, since removing it is stop's job.
func (m *Manager) Status(ctx context.Context, id string) (Status, error) {
	p, err := PathsFor(id)
	if err != nil {
		return Status{}, err
	}
	rec, found, err := ReadRecord(id)
	switch {
	case err != nil:
		return Status{Account: id, State: StateBroken, Why: err.Error(), Paths: p}, nil
	case !found:
		return Status{Account: id, State: StateStopped, Paths: p}, nil
	}
	return m.statusOf(ctx, rec, p), nil
}

func (m *Manager) statusOf(ctx context.Context, rec Record, p Paths) Status {
	st := Status{Account: rec.Account, Record: rec, Paths: p}
	st.State, st.Why = m.standing(ctx, rec)
	if st.State == StateRunning {
		st.Uptime = m.o.Now().Sub(rec.StartedAt)
	}
	return st
}

// StatusAll looks at every gateway that has a record, sorted by account.
func (m *Manager) StatusAll(ctx context.Context) ([]Status, error) {
	entries, err := List()
	if err != nil {
		return nil, err
	}
	var out []Status
	for _, e := range entries {
		p, err := PathsFor(e.Account)
		if err != nil {
			continue
		}
		if e.Err != nil {
			out = append(out, Status{Account: e.Account, State: StateBroken, Why: e.Err.Error(), Paths: p})
			continue
		}
		out = append(out, m.statusOf(ctx, e.Record, p))
	}
	return out, nil
}
