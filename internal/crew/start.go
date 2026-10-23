package crew

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// StartRequest is what Start needs to know about the account: its id and its
// folder, which the caller has already checked is signed in.
type StartRequest struct {
	Account string
	Folder  string
}

// StartResult says what Start did.
type StartResult struct {
	Record Record
	// Already is true when the account's gateway was running, and nothing was started.
	Already bool
	// Removed names the variables that were not passed on to the gateway.
	Removed []string
	// Warning is set when the gateway runs but did not answer in time.
	Warning string
}

// StartFailure is a gateway that did not come up. Output is the end of what it
// printed, with token values hidden.
type StartFailure struct {
	Reason string
	Output []string
}

// Error gives the reason; the output is for the caller to show beside it.
func (e *StartFailure) Error() string { return e.Reason }

// Start runs a gateway for the account, detached, unless one of this account is
// running already. the whole decision, from the port to the record, is made
// under one lock, so two starts at once cannot be given the same port. the wait
// for the gateway to answer is outside it.
func (m *Manager) Start(ctx context.Context, req StartRequest) (StartResult, error) {
	p, err := PathsFor(req.Account)
	if err != nil {
		return StartResult{}, err
	}
	if !filepath.IsAbs(req.Folder) {
		return StartResult{}, fmt.Errorf("the account folder must be an absolute path, got %q", req.Folder)
	}
	var res StartResult
	err = m.locked(func() (err error) {
		res, err = m.launch(ctx, req, p)
		return err
	})
	if err != nil {
		return res, err
	}
	return m.awaitReady(ctx, p, res)
}

func (m *Manager) launch(ctx context.Context, req StartRequest, p Paths) (StartResult, error) {
	if res, done, err := m.alreadyRunning(ctx, req.Account); done || err != nil {
		return res, err
	}
	bin, err := m.o.Binary()
	if err != nil {
		return StartResult{}, err
	}
	port, err := m.allocate()
	if err != nil {
		return StartResult{}, err
	}
	if err := os.MkdirAll(p.Home, 0o700); err != nil {
		return StartResult{}, err
	}
	claude, err := m.o.ClaudeBinary()
	if err != nil {
		return StartResult{}, err
	}
	env, removed := GatewayEnv(m.o.Environ(), req.Folder, claude, p, port)
	version, err := m.version(ctx, bin, env, p)
	if err != nil {
		return StartResult{}, err
	}
	if err := pinDefaults(p); err != nil {
		return StartResult{}, err
	}

	// the run before keeps its output, in case that is what went wrong
	if _, err := os.Stat(p.Log); err == nil {
		_ = os.Rename(p.Log, p.PrevLog)
	}
	args := []string{"gateway", "--port", strconv.Itoa(port), "--no-open", "--no-tunnel"}
	pid, err := m.o.Runner.Start(Spec{Bin: bin, Args: args, Env: env, Dir: p.Home}, p.Log)
	if err != nil {
		return StartResult{}, fmt.Errorf("could not start the gateway: %w", err)
	}
	proc, err := Probe(ctx, m.o.Runner, m.o.Environ(), pid)
	switch {
	case err != nil:
		// this pid is the child that was just started, so it is safe to end it,
		// and it must not be left running without a record
		_ = m.o.Signal(pid)
		return StartResult{}, fmt.Errorf("started the gateway, but ps could not tell about it, so it was stopped again: %w", err)
	case !proc.Alive:
		return StartResult{}, m.failure(p, "the gateway exited right after it was started")
	}
	rec := Record{
		Account: req.Account, PID: pid, Port: port, Home: p.Home, Version: version,
		StartedAt: m.o.Now(), ProcStart: proc.Start,
	}
	if err := WriteRecord(req.Account, rec); err != nil {
		_ = m.o.Signal(pid)
		return StartResult{}, fmt.Errorf("could not write the gateway record, so the gateway was stopped again: %w", err)
	}
	return StartResult{Record: rec, Removed: removed}, nil
}

// alreadyRunning looks at the record the account has. a gateway that is there
// is reported, one that is gone has its record removed, and one that cannot be
// confirmed is an error: a second gateway is not started over it.
func (m *Manager) alreadyRunning(ctx context.Context, id string) (res StartResult, done bool, err error) {
	rec, found, err := ReadRecord(id)
	if err != nil {
		return StartResult{}, false, fmt.Errorf("%w. check that no gateway of this account is running, then remove that file", err)
	}
	if !found {
		return StartResult{}, false, nil
	}
	switch state, why := m.standing(ctx, rec); state {
	case StateRunning:
		return StartResult{Record: rec, Already: true}, true, nil
	case StateStale:
		return StartResult{}, false, RemoveRecord(id)
	default:
		return StartResult{}, false, fmt.Errorf("%s. clalt will neither start a second gateway nor touch that process", why)
	}
}

// allocate gives the first port that no other gateway has recorded, whether
// that gateway is running or not, and that nothing listens on.
func (m *Manager) allocate() (int, error) {
	entries, err := List()
	if err != nil {
		return 0, err
	}
	taken := map[int]bool{}
	for _, e := range entries {
		if e.Err == nil {
			taken[e.Record.Port] = true
		}
	}
	return AllocatePort(m.o.FirstPort, m.o.LastPort, taken, m.o.Busy)
}

// awaitReady waits until the gateway answers on its port. one that ends
// meanwhile is a failure and loses its record. one that is alive but silent is
// a warning: the first start of a home builds its databases.
func (m *Manager) awaitReady(ctx context.Context, p Paths, res StartResult) (StartResult, error) {
	rec := res.Record
	deadline := time.Now().Add(m.o.ReadyWait)
	for {
		if proc, err := Probe(ctx, m.o.Runner, m.o.Environ(), rec.PID); err == nil && !proc.Alive {
			m.forget(rec)
			return res, m.failure(p, "the gateway exited while it was starting")
		}
		if m.o.Ready(ctx, rec.Port) {
			return m.checkBind(ctx, res)
		}
		if !time.Now().Before(deadline) {
			res.Warning = fmt.Sprintf("the gateway is running but has not answered on port %d yet: 'clalt crew logs' shows what it printed", rec.Port)
			return res, nil
		}
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		case <-time.After(m.o.Poll):
		}
	}
}

// forget removes the record of a gateway that was seen ending, unless a newer
// start has replaced it meanwhile.
func (m *Manager) forget(rec Record) {
	_ = m.locked(func() error {
		if now, found, err := ReadRecord(rec.Account); err == nil && found && now.PID == rec.PID {
			return RemoveRecord(rec.Account)
		}
		return nil
	})
}

// failure is a gateway that did not come up, with the end of its output.
func (m *Manager) failure(p Paths, reason string) *StartFailure {
	out, _ := Tail(p.Log, 10)
	return &StartFailure{Reason: reason, Output: out}
}
