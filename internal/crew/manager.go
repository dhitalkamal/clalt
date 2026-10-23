package crew

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/dhitalkamal/clalt/internal/claudecli"
	"github.com/dhitalkamal/clalt/internal/fslock"
)

// lockTimeout is how long start and stop wait for each other. a start can hold
// the lock for as long as the gateway takes to answer.
const lockTimeout = 60 * time.Second

// Options are what a Manager takes from outside. a field left at its zero value
// gets the real thing, so only a test sets any of them.
type Options struct {
	Runner Runner
	Binary func() (string, error)
	// ClaudeBinary resolves the claude that the gateway's adapter runs
	ClaudeBinary func() (string, error)
	// Listeners lists the addresses a process listens on, lsof by default
	Listeners func(ctx context.Context, pid int) ([]string, error)
	// AllowUntested lifts the version check, see AllowUntestedVar
	AllowUntested bool
	Environ       func() []string
	Now           func() time.Time
	// Busy says whether something listens on a port, Ready whether a gateway
	// answers on it, and Signal asks a process to stop.
	Busy   func(port int) bool
	Ready  func(ctx context.Context, port int) bool
	Signal func(pid int) error
	// the ports a gateway can get, and how long Start waits for a gateway to
	// answer, Stop for a process to end, and how often each looks again
	FirstPort, LastPort       int
	ReadyWait, StopWait, Poll time.Duration
}

// Manager starts, stops and looks at the gateways clalt runs. it keeps no
// state of its own: everything it knows is in the records on disk.
type Manager struct{ o Options }

// NewManager makes a Manager, with the real thing for every option left empty.
func NewManager(o Options) *Manager {
	if o.Runner == nil {
		o.Runner = ExecRunner{}
	}
	if o.Binary == nil {
		o.Binary = Binary
	}
	if o.ClaudeBinary == nil {
		o.ClaudeBinary = claudecli.Binary
	}
	o.AllowUntested = o.AllowUntested || os.Getenv(AllowUntestedVar) == "1"
	if o.Environ == nil {
		o.Environ = os.Environ
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Busy == nil {
		o.Busy = PortBusy
	}
	if o.Ready == nil {
		o.Ready = DialReady
	}
	if o.Signal == nil {
		o.Signal = terminate
	}
	if o.FirstPort == 0 {
		o.FirstPort = FirstPort
	}
	if o.LastPort == 0 {
		o.LastPort = LastPort
	}
	if o.ReadyWait == 0 {
		o.ReadyWait = 30 * time.Second
	}
	if o.StopWait == 0 {
		o.StopWait = 30 * time.Second
	}
	if o.Poll == 0 {
		o.Poll = 250 * time.Millisecond
	}
	return &Manager{o: o}
}

// Binary resolves the kirocrew executable: CLALT_KIROCREW_BIN when set, else
// kirocrew on the PATH. looking it up first gives one clear error up front.
func Binary() (string, error) {
	name := os.Getenv("CLALT_KIROCREW_BIN")
	if name == "" {
		name = "kirocrew"
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("cannot run %q (install Kiro Crew, or point CLALT_KIROCREW_BIN at it): %w", name, err)
	}
	return path, nil
}

// DialReady says whether something accepts connections on the port of the
// loopback address, which is where a gateway listens.
func DialReady(ctx context.Context, port int) bool {
	d := net.Dialer{Timeout: 300 * time.Millisecond}
	c, err := d.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// locked runs fn while holding the one lock of start and stop.
func (m *Manager) locked(fn func() error) error {
	return fslock.WithLock(LockPath(), lockTimeout, fn)
}

// standing is where a recorded gateway stands. ps that cannot be asked is not
// a gateway that ended: it is one that cannot be confirmed.
func (m *Manager) standing(ctx context.Context, rec Record) (State, string) {
	proc, err := Probe(ctx, m.o.Runner, m.o.Environ(), rec.PID)
	if err != nil {
		return StateUnverified, err.Error()
	}
	return Judge(rec, proc)
}
