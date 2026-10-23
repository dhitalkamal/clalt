package wrapper

import (
	"errors"
	"os"
	"sync"
	"time"
)

var (
	errTerminated = errors.New("signal: terminated")
	errKilled     = errors.New("signal: killed")
)

type clockT struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clockT) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clockT) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// fakeProc is a claude that ends when it is told to, or never does.
type fakeProc struct {
	pid   int
	ended chan struct{}
	once  sync.Once
	err   error

	mu           sync.Mutex
	terminated   int
	killed       int
	signals      []os.Signal
	ignoreTerm   bool
	ignoreKill   bool
	terminateErr error
	quitOnTerm   bool
}

func newProc(pid int) *fakeProc { return &fakeProc{pid: pid, ended: make(chan struct{})} }

func (p *fakeProc) PID() int    { return p.pid }
func (p *fakeProc) Wait() error { <-p.ended; return p.err }

func (p *fakeProc) end(err error) {
	p.once.Do(func() { p.err = err; close(p.ended) })
}

func (p *fakeProc) Terminate() error {
	p.mu.Lock()
	p.terminated++
	ignore := p.ignoreTerm
	terr := p.terminateErr
	p.mu.Unlock()
	if terr != nil {
		return terr
	}
	if !ignore {
		if p.quitOnTerm {
			p.end(nil)
		} else {
			p.end(errTerminated)
		}
	}
	return nil
}

func (p *fakeProc) Kill() error {
	p.mu.Lock()
	p.killed++
	ignore := p.ignoreKill
	p.mu.Unlock()
	if !ignore {
		p.end(errKilled)
	}
	return nil
}

func (p *fakeProc) Signal(s os.Signal) error {
	p.mu.Lock()
	p.signals = append(p.signals, s)
	p.mu.Unlock()
	return nil
}

func (p *fakeProc) counts() (terminated, killed int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.terminated, p.killed
}

func (p *fakeProc) got() []os.Signal {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]os.Signal(nil), p.signals...)
}

type start struct {
	folder string
	args   []string
}

type fakeLauncher struct {
	mu     sync.Mutex
	starts []start
	procs  []*fakeProc
	fail   map[string]error // by folder
	spoil  func(p *fakeProc, n int)
}

func (l *fakeLauncher) Start(folder string, args []string) (Process, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.starts = append(l.starts, start{folder, append([]string(nil), args...)})
	if err := l.fail[folder]; err != nil {
		return nil, err
	}
	p := newProc(1000 + len(l.procs))
	if l.spoil != nil {
		l.spoil(p, len(l.procs))
	}
	l.procs = append(l.procs, p)
	return p, nil
}

func (l *fakeLauncher) started() []start {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]start(nil), l.starts...)
}

func (l *fakeLauncher) proc(i int) *fakeProc {
	l.mu.Lock()
	defer l.mu.Unlock()
	if i >= len(l.procs) {
		return nil
	}
	return l.procs[i]
}

func (p *fakeProc) terminateCalls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.terminated
}

func (p *fakeProc) EndedByTerminate(err error) bool { return err == errTerminated || err == errKilled }
