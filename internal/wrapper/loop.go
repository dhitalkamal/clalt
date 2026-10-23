package wrapper

import (
	"fmt"
	"os"
	"time"
)

// Session is the session the wrapper keeps going: its handle, the account it
// starts as (id and folder), where it runs, and the arguments claude was started
// with, which a resume carries over, see ResumeArgs.
type Session struct {
	Handle, Account, Folder, Cwd string
	Args                         []string
}

// Deps is everything the wrapper needs from outside. Terminal, Signals, Notice
// and Now may be left out.
type Deps struct {
	Registry Registry
	Feed     Feed
	Accounts Accounts
	Launcher Launcher
	Terminal Terminal
	// Signals are the signals clalt received, see relay.
	Signals <-chan os.Signal
	// Notice prints one line for the user. it is only called while no claude
	// runs, since claude owns the terminal until then.
	Notice func(string)
	Now    func() time.Time
	Config Config
}

type loop struct {
	s   Session
	d   Deps
	cfg Config

	proc        Process
	done        chan error
	account     string
	folder      string
	procStarted time.Time
	lastMove    time.Time
	blocked     map[string]time.Time
	notes       []string
}

// Run starts claude for the session and keeps it going until claude ends, moving
// it to another account at a pause when it is asked to, see WriteRequest, or when
// auto-switch decides to, see Decide. it returns what the last claude ended with,
// or an error when a move left nothing that could be started.
func Run(s Session, d Deps) error {
	if d.Now == nil {
		d.Now = time.Now
	}
	l := &loop{s: s, d: d, cfg: d.Config.withDefaults(), account: s.Account, folder: s.Folder, blocked: map[string]time.Time{}}
	defer RemoveRequests(d.Registry.Dir(), s.Handle)
	if err := l.start(s.Folder, s.Args, Change{}); err != nil {
		return err
	}
	tick := time.NewTicker(l.cfg.Poll)
	defer tick.Stop()
	for {
		select {
		case err := <-l.done:
			l.flush()
			return err
		case sig := <-d.Signals:
			l.relay(sig)
		case <-tick.C:
			if exit, err := l.tick(); exit {
				l.flush()
				return err
			}
		}
	}
}

// start runs claude and tells the registry, with ch and the pid.
func (l *loop) start(folder string, args []string, ch Change) error {
	p, err := l.d.Launcher.Start(folder, args)
	if err != nil {
		return err
	}
	l.proc = p
	l.done = make(chan error, 1)
	go func(done chan<- error) { done <- p.Wait() }(l.done)
	l.procStarted = l.d.Now()
	ch.ClaudePID = p.PID()
	if err := l.d.Registry.Update(l.s.Handle, ch); err != nil {
		l.note("could not update the session record: %v", err)
	}
	return nil
}

// relay passes on what clalt was sent. a Ctrl-C reaches claude by itself, since
// both are in the terminal's foreground group, so the interrupt is not passed on.
// SIGTERM and SIGHUP are for clalt alone, and claude would be left running with
// nobody to answer to.
func (l *loop) relay(sig os.Signal) {
	if sig != os.Interrupt {
		_ = l.proc.Signal(sig)
	}
}

// note keeps a line for when the terminal is free, flush prints them.
func (l *loop) note(format string, args ...any) {
	l.notes = append(l.notes, fmt.Sprintf(format, args...))
}

func (l *loop) flush() {
	if l.d.Notice != nil {
		for _, n := range l.notes {
			l.d.Notice(n)
		}
	}
	l.notes = nil
}

// paused says whether the session is between replies: idle, for long enough that
// the next prompt is not on its way, and seen after this claude started, so that
// what the one before it left in the feed is never read.
func (l *loop) paused(info SessionInfo) bool {
	return info.State == Idle && info.UpdatedAt.After(l.procStarted) && l.d.Now().Sub(info.UpdatedAt) >= l.cfg.Settle
}

func (l *loop) record() (Record, bool) {
	recs, err := l.d.Registry.List()
	if err != nil {
		return Record{}, false
	}
	for _, r := range recs {
		if r.Handle == l.s.Handle {
			return r, true
		}
	}
	return Record{}, false
}
