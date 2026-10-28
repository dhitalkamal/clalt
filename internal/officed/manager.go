package officed

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// Child is a process the manager started.
type Child struct {
	PID int
	// Exited is closed when the process ends
	Exited <-chan struct{}
}

// Manager is on, off, open and status over the run file. every effect on the
// machine is a field, so tests fake them all.
type Manager struct {
	Dir     string
	GOOS    string
	Version string
	Out     io.Writer
	Now     func() time.Time
	// Verify is nil when the server in the run file answers as the clalt office
	// server of that pid, with that token. it is the only thing that lets a pid
	// be signalled.
	Verify func(Run) error
	Alive  func(pid int) bool
	// Stop asks the pid politely to end, Spawn starts the detached server
	Stop  func(pid int) error
	Spawn func(logPath string) (Child, error)
	Open  func(url string) error
	Sleep func(time.Duration)
}

const (
	poll      = 100 * time.Millisecond
	startWait = 10 * time.Second
	stopWait  = 5 * time.Second
)

func (m Manager) printf(format string, args ...any) { fmt.Fprintf(m.Out, format, args...) }

// Running is the run file when its server verifiably is a clalt office server.
func (m Manager) Running() (Run, bool) {
	r, err := ReadRun(m.Dir)
	if err != nil || !m.Alive(r.PID) || m.Verify(r) != nil {
		return r, false
	}
	return r, true
}

// open opens the link, a browser that fails to open is only a warning.
func (m Manager) open(r Run) {
	if err := m.Open(r.Link()); err != nil {
		m.printf("could not open a browser (%v), run: clalt office open --link\n", err)
	}
}

// On starts the server in the background and opens it, or just opens it.
func (m Manager) On() error {
	if m.GOOS == "windows" {
		return errors.New("the background office is not available on Windows yet: run clalt office in a terminal")
	}
	if r, ok := m.Running(); ok {
		m.printf("clalt office is already running on port %d (pid %d)\n", r.Port, r.PID)
		m.open(r)
		return nil
	}
	if _, err := ReadRun(m.Dir); err == nil {
		RemoveRun(m.Dir, 0)
		m.printf("removed a stale run file\n")
	}
	if err := os.MkdirAll(m.Dir, 0o700); err != nil {
		return err
	}
	log := LogPath(m.Dir)
	_ = RotateLog(log, MaxLog)
	child, err := m.Spawn(log)
	if err != nil {
		return fmt.Errorf("could not start clalt office: %w", err)
	}
	for waited := time.Duration(0); waited < startWait; waited += poll {
		if r, err := ReadRun(m.Dir); err == nil && r.PID == child.PID && m.Verify(r) == nil {
			m.printf("clalt office is on at port %d (pid %d)\n", r.Port, r.PID)
			m.open(r)
			return nil
		}
		select {
		case <-child.Exited:
			return startFailure(log)
		default:
		}
		m.Sleep(poll)
	}
	return fmt.Errorf("clalt office did not come up in %s, see %s", startWait, log)
}

func startFailure(log string) error {
	if l := LastErrorLine(log); l != "" {
		return fmt.Errorf("clalt office stopped right after it started: %s", l)
	}
	return fmt.Errorf("clalt office stopped right after it started, see %s", log)
}

// Off stops the verified server. a run file whose pid fails the check is
// removed and nothing is signalled.
func (m Manager) Off() error {
	r, err := ReadRun(m.Dir)
	if err != nil {
		m.printf("clalt office is not running\n")
		return nil
	}
	if !m.Alive(r.PID) || m.Verify(r) != nil {
		RemoveRun(m.Dir, 0)
		m.printf("the run file was stale (no clalt office answers for pid %d on port %d): removed it, nothing was stopped\n", r.PID, r.Port)
		return nil
	}
	if err := m.Stop(r.PID); err != nil {
		return fmt.Errorf("could not stop pid %d: %w", r.PID, err)
	}
	for waited := time.Duration(0); waited < stopWait; waited += poll {
		if !m.Alive(r.PID) {
			RemoveRun(m.Dir, 0)
			m.printf("clalt office is off\n")
			return nil
		}
		m.Sleep(poll)
	}
	return fmt.Errorf("clalt office (pid %d) did not stop in %s, it is still running", r.PID, stopWait)
}

// OpenLink opens the running server in the browser.
func (m Manager) OpenLink() error {
	r, ok := m.Running()
	if !ok {
		return errors.New("clalt office is off: start it with clalt office on")
	}
	return m.Open(r.Link())
}

// Status says whether it is on. the link carries the token, so only withLink
// prints it.
func (m Manager) Status(withLink bool) error {
	r, ok := m.Running()
	if !ok {
		m.printf("clalt office: off\n")
	} else {
		m.printf("clalt office: on\nport:    %d\npid:     %d\nuptime:  %s\nversion: %s\n", r.Port, r.PID, uptime(m.Now().Sub(r.StartedAt)), r.Version)
		if withLink {
			m.printf("link:    %s\n", r.Link())
		} else {
			m.printf("link:    run clalt office status --link to print it (it carries the token)\n")
		}
	}
	if l := LastErrorLine(LogPath(m.Dir)); l != "" {
		m.printf("last error line: %s\n", l)
	}
	return nil
}

// uptime is 2d3h, 1h30m, 5m or 12s: the two biggest units.
func uptime(d time.Duration) string {
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
	case d >= time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%ds", max(0, int(d.Seconds())))
}
