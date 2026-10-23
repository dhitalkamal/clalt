package live

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/claudecli"
	"github.com/dhitalkamal/clalt/internal/dashboard"
	"github.com/dhitalkamal/clalt/internal/usage"
)

type fakeHost struct {
	mu       sync.Mutex
	signIns  map[string]SignIn // by account id
	checkErr error
	gate     chan struct{} // when set, checks wait for it
	checks   atomic.Int32
	version  string
	verErr   error
	links    bool
	linksErr error
	records  map[string][]usage.Record
	usageErr error
	scans    atomic.Int32
	signOuts []string
	signErr  error
	launches []string
}

func (h *fakeHost) CheckSignIn(acc account.Account) (SignIn, error) {
	h.checks.Add(1)
	h.mu.Lock()
	gate, in, err := h.gate, h.signIns[acc.Name], h.checkErr
	h.mu.Unlock()
	if gate != nil {
		<-gate
	}
	return in, err
}

func (h *fakeHost) ClaudeVersion() (string, error)   { return h.version, h.verErr }
func (h *fakeHost) LinksInSync(string) (bool, error) { return h.links, h.linksErr }

func (h *fakeHost) Usage() (map[string][]usage.Record, error) {
	h.scans.Add(1)
	return h.records, h.usageErr
}

func (h *fakeHost) SignOut(acc account.Account) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.signOuts = append(h.signOuts, acc.Name)
	return h.signErr
}

func (h *fakeHost) note(what string, acc account.Account) dashboard.Launch {
	h.mu.Lock()
	h.launches = append(h.launches, what+" "+acc.Label())
	h.mu.Unlock()
	return &runLaunch{}
}

func (h *fakeHost) Run(acc account.Account) dashboard.Launch   { return h.note("run", acc) }
func (h *fakeHost) Login(acc account.Account) dashboard.Launch { return h.note("login", acc) }

type runLaunch struct {
	ran int
	err error
}

func (l *runLaunch) Run() error          { l.ran++; return l.err }
func (l *runLaunch) SetStdin(io.Reader)  {}
func (l *runLaunch) SetStdout(io.Writer) {}
func (l *runLaunch) SetStderr(io.Writer) {}

type fakeFeed struct {
	limits    Limits
	have      bool
	readings  []Reading
	installed bool
	err       error
}

func (f fakeFeed) Limits(context.Context, string) (Limits, bool, error) {
	return f.limits, f.have, f.err
}
func (f fakeFeed) Readings(context.Context, string, time.Time) ([]Reading, error) {
	return f.readings, nil
}
func (f fakeFeed) Installed(string) bool { return f.installed }

type fakeSessions struct {
	list    []Session
	err     error
	stopped []string
}

func (f *fakeSessions) Running(context.Context) ([]Session, error) { return f.list, f.err }
func (f *fakeSessions) Stop(_ context.Context, id, handle string) error {
	f.stopped = append(f.stopped, id+"/"+handle)
	return nil
}

type failGateways struct{}

func (failGateways) Running(context.Context) ([]Gateway, error) { return nil, errors.New("port busy") }

// rig is an adapter on a real store in a temp dir
type rig struct {
	t     *testing.T
	store *account.Store
	host  *fakeHost
	clock *fakeClock
	cfg   Config
}

func newRig(t *testing.T) *rig {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	store := account.New(filepath.Join(dir, "clalt", "state.json"), filepath.Join(dir, "clalt", "switches.jsonl"))
	host := &fakeHost{version: "2.1.303", signIns: map[string]SignIn{}, links: true}
	clock := &fakeClock{t: epoch}
	return &rig{t: t, store: store, host: host, clock: clock,
		cfg: Config{Store: store, Host: host, Now: clock.now, Home: "/h", Grace: 500 * time.Millisecond}}
}

func (r *rig) adapter() *Adapter { return New(r.cfg) }

// signedIn makes an account with a recorded sign-in and has the host agree
func (r *rig) signedIn(label, email string) string {
	r.t.Helper()
	acc, err := r.store.Create(label, func(string) (bool, error) { return false, nil })
	if err != nil {
		r.t.Fatal(err)
	}
	if err := r.store.SetIdentity(acc.Name, account.Identity{Email: email, Org: "Org", Plan: "max", SignedInAt: epoch.Add(-48 * time.Hour)}); err != nil {
		r.t.Fatal(err)
	}
	r.host.signIns[acc.Name] = SignIn{OK: true, Email: email, Org: "Org", Plan: "max"}
	return acc.Name
}

func (r *rig) load(a *Adapter) dashboard.Snapshot {
	r.t.Helper()
	snap, err := a.Load(context.Background())
	if err != nil {
		r.t.Fatal(err)
	}
	return snap
}

func byLabel(snap dashboard.Snapshot, label string) dashboard.Account {
	for _, a := range snap.Accounts {
		if a.Label == label {
			return a
		}
	}
	return dashboard.Account{}
}

var _ = claudecli.ReasonNone

func statePathOf(t *testing.T) string {
	t.Helper()
	return filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "clalt", "state.json")
}

func writeState(t *testing.T, body string) {
	t.Helper()
	p := statePathOf(t)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
