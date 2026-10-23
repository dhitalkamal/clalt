package live

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/claudecli"
	"github.com/dhitalkamal/clalt/internal/dashboard"
)

// find is the visible account with this id
func (a *Adapter) find(id string) (account.Account, error) { return a.lookup(id, false) }

// lookup is the account with this id. a removed one counts only when
// withRemoved is set: delete and sign-in are the two things a removed account
// can be given.
func (a *Adapter) lookup(id string, withRemoved bool) (account.Account, error) {
	all, err := a.cfg.Store.List()
	if err != nil {
		return account.Account{}, err
	}
	for _, acc := range all {
		if acc.Name == id && (withRemoved || !acc.Meta.Hidden) {
			return acc, nil
		}
	}
	return account.Account{}, fmt.Errorf("no account with the id %q", id)
}

// launched runs l and then forgets what was known: a session or a sign-in
// changes the login and the usage
type launched struct {
	dashboard.Launch
	after func()
}

func (l launched) Run() error {
	defer l.after()
	return l.Launch.Run()
}

func (l launched) StartError() error {
	if s, ok := l.Launch.(interface{ StartError() error }); ok {
		return s.StartError()
	}
	return nil
}

func (a *Adapter) launch(id string, withRemoved bool, make func(account.Account) dashboard.Launch) (dashboard.Launch, error) {
	acc, err := a.lookup(id, withRemoved)
	if err != nil {
		return nil, err
	}
	// a label is the argument of the command, and one that starts with a dash
	// would be read as a flag. old state files were never checked for them.
	if strings.HasPrefix(acc.Label(), "-") {
		return nil, fmt.Errorf("the label %q starts with a dash. rename it first", acc.Label())
	}
	l := make(acc)
	if l == nil {
		return nil, nil
	}
	return launched{Launch: l, after: func() { a.forget(acc.Name) }}, nil
}

// forget makes the next load ask again about the account and read the usage
func (a *Adapter) forget(id string) {
	a.mu.Lock()
	e := a.entries[id]
	a.mu.Unlock()
	if e != nil {
		e.c.invalidate()
	}
	a.ledger.invalidate()
}

// Run starts claude as the account, through the code of `clalt run`.
func (a *Adapter) Run(_ context.Context, id string) (dashboard.Launch, error) {
	return a.launch(id, false, a.cfg.Host.Run)
}

// SignIn signs the account in, through the code of `clalt login`. that also
// brings a removed account back.
func (a *Adapter) SignIn(_ context.Context, id string) (dashboard.Launch, error) {
	return a.launch(id, true, a.cfg.Host.Login)
}

// Rename changes the label and nothing else.
func (a *Adapter) Rename(_ context.Context, id, label string) error {
	if _, err := a.find(id); err != nil {
		return err
	}
	return a.cfg.Store.Relabel(id, label)
}

// SetBudget sets both caps, 0 meaning none.
func (a *Adapter) SetBudget(_ context.Context, id string, daily, weekly float64) error {
	for _, v := range []float64{daily, weekly} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return fmt.Errorf("a budget is an amount of dollars, 0 or more, not %v", v)
		}
	}
	if _, err := a.find(id); err != nil {
		return err
	}
	return a.cfg.Store.SetBudget(id, daily, weekly)
}

// SignOut signs the account out of claude. the cached sign-in is replaced only
// when that worked.
func (a *Adapter) SignOut(_ context.Context, id string) error {
	acc, err := a.find(id)
	if err != nil {
		return err
	}
	if err := a.cfg.Host.SignOut(acc); err != nil {
		return err
	}
	a.entry(acc).c.set(SignIn{Reason: claudecli.ReasonNotSignedIn})
	return nil
}

// Delete deletes the account and its folder through the host, removed ones
// included. the host refuses a live session, a shared folder and a failed
// sign-out. the cached state of the account goes with it.
func (a *Adapter) Delete(_ context.Context, id string) error {
	acc, err := a.lookup(id, true)
	if err != nil {
		return err
	}
	if err := a.cfg.Host.Delete(acc); err != nil {
		return err
	}
	a.mu.Lock()
	delete(a.entries, acc.Name)
	a.mu.Unlock()
	a.ledger.invalidate()
	return nil
}

// StopSession stops a running session through the registry.
func (a *Adapter) StopSession(ctx context.Context, id, sessionID string) error {
	return a.cfg.Sessions.Stop(ctx, id, sessionID)
}
