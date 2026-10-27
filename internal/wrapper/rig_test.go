package wrapper

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/pick"
)

type fakeRegistry struct {
	mu        sync.Mutex
	dir       string
	recs      []Record
	updates   []Change
	updateErr error
}

func (r *fakeRegistry) List() ([]Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Record(nil), r.recs...), nil
}
func (r *fakeRegistry) Dir() string { return r.dir }
func (r *fakeRegistry) Update(handle string, c Change) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, c)
	return r.updateErr
}
func (r *fakeRegistry) setSessionID(id string) {
	r.mu.Lock()
	r.recs[0].ClaudeSessionID = id
	r.mu.Unlock()
}
func (r *fakeRegistry) changes() []Change {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Change(nil), r.updates...)
}

type fakeFeed struct {
	mu       sync.Mutex
	sessions map[string]SessionInfo
	limits   map[string]pick.Limits
}

func (f *fakeFeed) Session(h string) (SessionInfo, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[h]
	return s, ok
}
func (f *fakeFeed) Limits(id string) (pick.Limits, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	l, ok := f.limits[id]
	return l, ok
}
func (f *fakeFeed) set(h string, s SessionInfo) {
	f.mu.Lock()
	f.sessions[h] = s
	f.mu.Unlock()
}

type fakeAccounts struct {
	mu       sync.Mutex
	cands    []pick.Candidate
	prepare  func(id, cwd string) (Prepared, error)
	prepared []string
	candErr  error
}

func (a *fakeAccounts) Candidates() ([]pick.Candidate, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.candErr != nil {
		return nil, a.candErr
	}
	return append([]pick.Candidate(nil), a.cands...), nil
}
func (a *fakeAccounts) Prepare(id, cwd string) (Prepared, error) {
	a.mu.Lock()
	a.prepared = append(a.prepared, id)
	fn := a.prepare
	a.mu.Unlock()
	if fn != nil {
		return fn(id, cwd)
	}
	return Prepared{Label: id, Folder: "/f/" + id}, nil
}
func (a *fakeAccounts) calls() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.prepared...)
}

type fakeTerminal struct {
	mu       sync.Mutex
	restored []bool
}

func (t *fakeTerminal) Restore(forced bool) {
	t.mu.Lock()
	t.restored = append(t.restored, forced)
	t.mu.Unlock()
}
func (t *fakeTerminal) got() []bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]bool(nil), t.restored...)
}

type notices struct {
	mu    sync.Mutex
	lines []string
}

func (n *notices) add(s string) {
	n.mu.Lock()
	n.lines = append(n.lines, s)
	n.mu.Unlock()
}
func (n *notices) all() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.lines...)
}

// rig is a session on work, with personal to move to, wired to fakes.
type rig struct {
	t      *testing.T
	clk    *clockT
	reg    *fakeRegistry
	feed   *fakeFeed
	acc    *fakeAccounts
	launch *fakeLauncher
	term   *fakeTerminal
	notes  *notices
	sigs   chan os.Signal
	sess   Session
	cfg    Config
	done   chan error
}

func newRig(t *testing.T) *rig {
	t.Helper()
	r := &rig{
		t: t, clk: &clockT{t: clock},
		reg:  &fakeRegistry{dir: t.TempDir(), recs: []Record{{Handle: handle, Account: "work", Folder: "/f/work", ClaudeSessionID: "sid-1", Cwd: "/x"}}},
		feed: &fakeFeed{sessions: map[string]SessionInfo{}, limits: map[string]pick.Limits{}},
		acc: &fakeAccounts{cands: []pick.Candidate{
			{ID: "work", Label: "work", SignedIn: true}, {ID: "personal", Label: "personal", SignedIn: true},
		}},
		launch: &fakeLauncher{fail: map[string]error{}}, term: &fakeTerminal{}, notes: &notices{},
		sigs: make(chan os.Signal, 8), done: make(chan error, 1),
		sess: Session{Handle: handle, Account: "work", Folder: "/f/work", Cwd: "/x", Args: []string{"fix it", "--add-dir", "../a"}},
		cfg:  Config{Poll: 2 * time.Millisecond, Settle: time.Second, Grace: 60 * time.Millisecond, Cooldown: 30 * time.Minute},
	}
	return r
}

func (r *rig) deps() Deps {
	return Deps{
		Registry: r.reg, Feed: r.feed, Accounts: r.acc, Launcher: r.launch, Terminal: r.term,
		Signals: r.sigs, Notice: r.notes.add, Now: r.clk.Now, Config: r.cfg,
	}
}

// run starts the wrapper and waits until it has started the first claude.
func (r *rig) run() {
	r.t.Helper()
	go func() { r.done <- Run(r.sess, r.deps()) }()
	waitFor(r.t, "the first claude to start", func() bool { return len(r.launch.started()) >= 1 })
}

// idle makes the session look paused: idle for five seconds, seen after the start.
func (r *rig) idle() {
	r.clk.Advance(10 * time.Second)
	r.feed.set(handle, SessionInfo{State: Idle, UpdatedAt: r.clk.Now().Add(-5 * time.Second)})
}

func (r *rig) busy() {
	r.clk.Advance(10 * time.Second)
	r.feed.set(handle, SessionInfo{State: Busy, UpdatedAt: r.clk.Now()})
}

func (r *rig) request(to string) {
	r.t.Helper()
	if err := WriteRequest(r.reg.dir, handle, Request{To: to, RequestedAt: r.clk.Now()}); err != nil {
		r.t.Fatal(err)
	}
}

// result waits for the wrapper to end and gives what it returned.
func (r *rig) result() error {
	r.t.Helper()
	select {
	case err := <-r.done:
		return err
	case <-time.After(5 * time.Second):
		r.t.Fatal("the wrapper did not end")
		return nil
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// settle gives the loop time to act on what it was shown, for a test that
// expects it to do nothing.
func settle() { time.Sleep(60 * time.Millisecond) }

func (r *rig) noticeText() string { return fmt.Sprint(r.notes.all()) }
