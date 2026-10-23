package wrapper

import (
	"os"
	"os/exec"
)

// ExecProcess is a claude started with os/exec.
type ExecProcess struct{ cmd *exec.Cmd }

// StartExec starts cmd and returns it as a Process. the caller has attached the
// terminal to it already.
func StartExec(cmd *exec.Cmd) (Process, error) {
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &ExecProcess{cmd: cmd}, nil
}

// PID is the pid of claude.
func (p *ExecProcess) PID() int { return p.cmd.Process.Pid }

// Wait returns when claude has ended. it is called once.
func (p *ExecProcess) Wait() error { return p.cmd.Wait() }

// Terminate asks claude to end by itself, see terminate.
func (p *ExecProcess) Terminate() error { return terminate(p.cmd.Process) }

// Kill ends claude at once.
func (p *ExecProcess) Kill() error { return p.cmd.Process.Kill() }

// Signal passes a signal on to claude.
func (p *ExecProcess) Signal(s os.Signal) error { return p.cmd.Process.Signal(s) }

// EndedByTerminate: see terminated.
func (p *ExecProcess) EndedByTerminate(err error) bool { return terminated(err) }
