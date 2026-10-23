package dashboard

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

type loadedMsg struct {
	snap Snapshot
	err  error
	at   time.Time // when the read started, which is how old its data is
}

type tickMsg struct{}

func (m model) now() time.Time { return m.opt.Now() }

// the tick also redraws the "updated Ns ago" age, so it is at most a second
func (m model) tickCmd() tea.Cmd {
	return tea.Tick(min(time.Second, m.opt.Refresh), func(time.Time) tea.Msg { return tickMsg{} })
}

// protect runs fn and turns a panic in it into an error, so a bug in an
// adapter shows up as a message and not as the end of the program
func protect(what string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s panicked: %v", what, r)
		}
	}()
	return fn()
}

// logf hands a failure to the logger, if there is one
func (m model) logf(format string, args ...any) {
	if m.opt.Logger != nil {
		m.opt.Logger(fmt.Sprintf(format, args...))
	}
}

// timeoutError is what a load that ran out of time says
func timeoutError(limit time.Duration) error {
	return fmt.Errorf("load timed out after %s", limit)
}

// readEnded reads a load's outcome against its context. a deadline that passed
// is a timeout whatever the source said, since a read that ends with no error
// after the deadline may be partial.
func readEnded(ctx context.Context, err error, limit time.Duration) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) && (err == nil || errors.Is(err, context.DeadlineExceeded)) {
		return timeoutError(limit)
	}
	if err == nil {
		err = ctx.Err()
	}
	return err
}

// loadCmd calls the source inside a command, so the UI loop never waits on it.
// the call has its own goroutine, so a source that ignores its context cannot
// hold the dashboard past the timeout.
func (m model) loadCmd() tea.Cmd {
	parent, src, limit, now := m.ctx, m.src, m.opt.LoadTimeout, m.now
	return func() tea.Msg {
		started := now()
		ctx, cancel := context.WithTimeout(parent, limit)
		defer cancel()
		type result struct {
			snap Snapshot
			err  error
		}
		// room for the answer of a read nobody waits for any more
		done := make(chan result, 1)
		go func() {
			var r result
			r.err = protect("load", func() (err error) { r.snap, err = src.Load(ctx); return })
			done <- r
		}()
		select {
		case r := <-done:
			return loadedMsg{snap: r.snap, err: readEnded(ctx, r.err, limit), at: started}
		case <-ctx.Done():
			return loadedMsg{err: readEnded(ctx, nil, limit), at: started}
		}
	}
}

func (m model) startLoad() (model, tea.Cmd) {
	m.loading = true
	m.loadStart = m.now()
	return m, m.loadCmd()
}

// reload asks for a fresh read after something changed. if a load is already
// running it runs once more when that one finishes.
func (m model) reload() (model, tea.Cmd) {
	if m.loading {
		m.dirty = true
		return m, nil
	}
	return m.startLoad()
}

// onTick starts a refresh when one is due and none is in flight, so a slow
// source never stacks loads up
func (m model) onTick() (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{m.tickCmd()}
	if !m.loading && !m.now().Before(m.nextLoad) {
		var load tea.Cmd
		m, load = m.startLoad()
		cmds = append(cmds, load)
	}
	return m, tea.Batch(cmds...)
}

// checkSnapshot refuses a read whose accounts cannot be told apart, since every
// action addresses an account by its id
func checkSnapshot(snap Snapshot) error {
	seen := map[string]int{}
	for i, a := range snap.Accounts {
		if a.ID == "" {
			return fmt.Errorf("account %d (%q) has no id", i+1, a.Label)
		}
		if first, dup := seen[a.ID]; dup {
			return fmt.Errorf("accounts %d and %d share the id %q", first+1, i+1, a.ID)
		}
		seen[a.ID] = i
	}
	for i, r := range snap.Removed {
		if r.ID == "" {
			return fmt.Errorf("removed account %d (%q) has no id", i+1, r.Label)
		}
		if _, dup := seen[r.ID]; dup {
			return fmt.Errorf("the removed account %d shares the id %q with another", i+1, r.ID)
		}
		seen[r.ID] = -1
	}
	return nil
}

// shownWarnings leaves out the blank ones
func shownWarnings(ws []string) []string {
	var out []string
	for _, w := range ws {
		if strings.TrimSpace(w) != "" {
			out = append(out, w)
		}
	}
	return out
}

func (m model) onLoaded(msg loadedMsg) (model, tea.Cmd) {
	m.loading = false
	m.nextLoad = m.now().Add(m.opt.Refresh)
	if msg.err == nil {
		msg.snap = cleanSnapshot(msg.snap)
		msg.err = checkSnapshot(msg.snap)
	}
	if msg.err != nil {
		m.loadErr = msg.err
		m.logf("load failed: %v", msg.err)
	} else {
		msg.snap.Warnings = shownWarnings(msg.snap.Warnings)
		m.loadErr, m.snap, m.have, m.loadedAt = nil, msg.snap, true, msg.at
		if msg.at.IsZero() {
			m.loadedAt = m.now()
		}
		m = m.refilter()
	}
	if m.dirty {
		m.dirty = false
		return m.startLoad()
	}
	return m, nil
}

func (m model) running() int {
	n := 0
	for _, a := range m.snap.Accounts {
		n += len(a.Sessions)
	}
	return n
}
