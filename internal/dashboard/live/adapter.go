package live

import (
	"sync"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/dashboard"
)

// statusWorkers is how many `claude auth status` calls run at once
const statusWorkers = 4

// Config is everything an Adapter needs. Store and Host are required, the rest
// has a default.
type Config struct {
	Store    *account.Store
	Host     Host
	Feed     Feed     // nil: NoFeed
	Sessions Sessions // nil: NoSessions
	Now      func() time.Time
	Home     string // the user's home, shown as ~ in folders
	// DefaultHome is the id of the account that plain claude (~/.claude) is
	// signed in as, or empty. nil means none.
	DefaultHome func() string

	// Grace is the longest a load waits for what a background read has not
	// brought yet. SignInTTL, UsageTTL and VersionTTL say how old an answer
	// gets before it is read again.
	Grace      time.Duration
	SignInTTL  time.Duration
	UsageTTL   time.Duration
	VersionTTL time.Duration
}

func (c Config) withDefaults() Config {
	if c.Feed == nil {
		c.Feed = NoFeed{}
	}
	if c.Sessions == nil {
		c.Sessions = NoSessions{}
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	for _, d := range []struct {
		v   *time.Duration
		def time.Duration
	}{{&c.Grace, 3 * time.Second}, {&c.SignInTTL, time.Minute}, {&c.UsageTTL, 30 * time.Second}, {&c.VersionTTL, 5 * time.Minute}} {
		if *d.v <= 0 {
			*d.v = d.def
		}
	}
	return c
}

type versionInfo struct {
	version string
	err     error // why this claude cannot be used
}

// signEntry is the sign-in cache of one account, with the account as the last
// load saw it
type signEntry struct {
	mu  sync.Mutex
	acc account.Account
	c   *cache[SignIn]
}

func (e *signEntry) account() account.Account {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.acc
}

// Adapter is the dashboard's Source and Actions on real data. it is safe for
// concurrent use: actions run while a load is in flight.
type Adapter struct {
	cfg Config

	mu      sync.Mutex
	entries map[string]*signEntry
	sem     chan struct{}
	ledger  *cache[*ledger]
	version *cache[versionInfo]
}

var (
	_ dashboard.Source  = (*Adapter)(nil)
	_ dashboard.Actions = (*Adapter)(nil)
)

// New makes an Adapter. it reads nothing until the first Load.
func New(cfg Config) *Adapter {
	cfg = cfg.withDefaults()
	a := &Adapter{cfg: cfg, entries: map[string]*signEntry{}, sem: make(chan struct{}, statusWorkers)}
	a.ledger = newCache(func() (*ledger, error) {
		recs, err := cfg.Host.Usage()
		if err != nil {
			return nil, err
		}
		return buildLedger(recs, cfg.Now(), cfg.Home), nil
	}, cfg.UsageTTL, cfg.Now)
	a.version = newCache(func() (versionInfo, error) {
		v, err := cfg.Host.ClaudeVersion()
		return versionInfo{version: v, err: err}, nil
	}, cfg.VersionTTL, cfg.Now)
	return a
}

// entry is the sign-in cache of an account, made on first use. the checks of
// all accounts share the workers.
func (a *Adapter) entry(acc account.Account) *signEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	e := a.entries[acc.Name]
	if e == nil {
		e = &signEntry{}
		e.c = newCache(func() (SignIn, error) {
			a.sem <- struct{}{}
			defer func() { <-a.sem }()
			return a.cfg.Host.CheckSignIn(e.account())
		}, a.cfg.SignInTTL, a.cfg.Now)
		a.entries[acc.Name] = e
	}
	e.mu.Lock()
	e.acc = acc
	e.mu.Unlock()
	return e
}
