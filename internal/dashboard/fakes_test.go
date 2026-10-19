package dashboard

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// fakeSource counts loads and can hold them open. each Load waits on gate when
// there is one, and gives up when the context ends.
type fakeSource struct {
	mu    sync.Mutex
	snap  Snapshot
	err   error
	calls atomic.Int32
	gate  chan struct{}
}

func (f *fakeSource) Load(ctx context.Context) (Snapshot, error) {
	f.calls.Add(1)
	f.mu.Lock()
	gate, snap, err := f.gate, f.snap, f.err
	f.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return Snapshot{}, ctx.Err()
		}
	}
	return snap, err
}

func (f *fakeSource) set(snap Snapshot, err error) {
	f.mu.Lock()
	f.snap, f.err = snap, err
	f.mu.Unlock()
}

// fakeActions records what the dashboard asked for, by account id, and the
// context each call got.
type fakeActions struct {
	mu    sync.Mutex
	calls []string
	ctxs  []context.Context
	cmd   *exec.Cmd
	err   error
}

func (f *fakeActions) record(ctx context.Context, format string, args ...any) {
	f.mu.Lock()
	f.calls = append(f.calls, fmt.Sprintf(format, args...))
	f.ctxs = append(f.ctxs, ctx)
	f.mu.Unlock()
}

func (f *fakeActions) log() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *fakeActions) contexts() []context.Context {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]context.Context(nil), f.ctxs...)
}

func (f *fakeActions) Run(ctx context.Context, id string) (*exec.Cmd, error) {
	f.record(ctx, "run %s", id)
	return f.cmd, f.err
}

func (f *fakeActions) SignIn(ctx context.Context, id string) (*exec.Cmd, error) {
	f.record(ctx, "signin %s", id)
	return f.cmd, f.err
}

func (f *fakeActions) Rename(ctx context.Context, id, label string) error {
	f.record(ctx, "rename %s %s", id, label)
	return f.err
}

func (f *fakeActions) SetBudget(ctx context.Context, id string, daily, weekly float64) error {
	f.record(ctx, "budget %s %.2f %.2f", id, daily, weekly)
	return f.err
}

func (f *fakeActions) SignOut(ctx context.Context, id string) error {
	f.record(ctx, "signout %s", id)
	return f.err
}

func (f *fakeActions) StopSession(ctx context.Context, id, sessionID string) error {
	f.record(ctx, "stop %s %s", id, sessionID)
	return f.err
}

// the commands that stand in for claude are this test binary running
// TestHelperProcess, so no test depends on a program being installed
const helperEnv = "CLALT_DASHBOARD_TEST_HELPER"

// TestHelperProcess is not a test. helperCmd runs it as a child process that
// exits with the status in the environment.
func TestHelperProcess(t *testing.T) {
	code := os.Getenv(helperEnv)
	if code == "" {
		return
	}
	// bubbletea v2.1.0 can race its renderer when a command from tea.ExecProcess
	// returns at once, which shows with go test -cpu 1. a short run avoids it.
	time.Sleep(30 * time.Millisecond)
	n, _ := strconv.Atoi(code)
	os.Exit(n)
}

func helperCmd(exit int) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	cmd.Env = append(os.Environ(), helperEnv+"="+strconv.Itoa(exit))
	// the program's input is a pipe in these tests, and exec would wait for a
	// pipe that never closes. an empty reader ends at once.
	cmd.Stdin = strings.NewReader("")
	return cmd
}

// withIDs gives each account an id that differs from its label, to prove that
// actions address accounts by id.
func withIDs(accts []Account) []Account {
	out := append([]Account(nil), accts...)
	for i := range out {
		out[i].ID = fmt.Sprintf("a%d", i+1)
	}
	return out
}

func fiveSnapshot() Snapshot { return Snapshot{Accounts: withIDs(fiveAccounts())} }

// testOptions are fast and quiet: ascii, no color, a short refresh
func testOptions() Options {
	return Options{ASCII: true, NoColor: true, Refresh: time.Hour, FlashFor: 3 * time.Second, LoadTimeout: 5 * time.Second}
}

// loadedModel is a model that has its size and the first snapshot, the state
// a real program reaches a moment after it starts.
func loadedModel(src *fakeSource, act *fakeActions, opt Options, w, h int) model {
	return loadedModelIn(context.Background(), src, act, opt, w, h)
}

// loadedModelIn is loadedModel with the context the program runs under
func loadedModelIn(ctx context.Context, src *fakeSource, act *fakeActions, opt Options, w, h int) model {
	m := newModel(ctx, src, act, opt)
	next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m = next.(model)
	msg := RunCmd(m.loadCmd(), time.Second)
	next, _ = m.Update(msg)
	return next.(model)
}
