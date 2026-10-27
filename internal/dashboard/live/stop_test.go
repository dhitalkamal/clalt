package live

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/dashboard"
	"github.com/dhitalkamal/clalt/internal/registry"
)

type stopRig struct {
	f      files
	s      *RegistrySessions
	proc   Proc
	probe  error
	signed []int
	rec    registry.Record
}

// newStopRig has a record whose claude is a believable child of the wrapper
func newStopRig(t *testing.T) *stopRig {
	t.Helper()
	r := &stopRig{f: newFiles(t)}
	r.rec = r.f.record(t, h1, "work", epoch, func(x *registry.Record) { x.PID = 4242 })
	r.proc = Proc{Alive: true, Start: epoch.Add(2 * time.Second), PPID: r.rec.WrapperPID, Command: "/usr/local/bin/claude --resume"}
	r.s = &RegistrySessions{Reg: r.f.reg, Feed: r.f.fd, Now: func() time.Time { return epoch.Add(time.Minute) },
		Probe:  func(context.Context, int) (Proc, error) { return r.proc, r.probe },
		Signal: func(pid int) error { r.signed = append(r.signed, pid); return nil }}
	return r
}

func refused(t *testing.T, r *stopRig, handle string) {
	t.Helper()
	err := r.s.Stop(context.Background(), "work", handle)
	if err == nil {
		t.Fatal("stop was not refused")
	}
	if _, ok := err.(dashboard.Notice); !ok {
		t.Errorf("a refusal should be a Notice, got %T: %v", err, err)
	}
	if len(r.signed) != 0 {
		t.Errorf("signalled %v on a refused stop", r.signed)
	}
}

func TestStopSignalsClaudesPidWhenEverythingMatches(t *testing.T) {
	r := newStopRig(t)
	if err := r.s.Stop(context.Background(), "work", h1); err != nil {
		t.Fatal(err)
	}
	if len(r.signed) != 1 || r.signed[0] != 4242 {
		t.Errorf("signalled %v, want claude's pid only", r.signed)
	}
}

func TestStopRefusesAnUnknownOrBadHandle(t *testing.T) {
	r := newStopRig(t)
	refused(t, r, h2)
	refused(t, r, "../etc")
}

func TestStopRefusesAnotherAccountsHandle(t *testing.T) {
	r := newStopRig(t)
	if err := r.s.Stop(context.Background(), "other", h1); err == nil || len(r.signed) != 0 {
		t.Errorf("err=%v signed=%v", err, r.signed)
	}
}

func TestStopRefusesADeadPid(t *testing.T) {
	r := newStopRig(t)
	r.proc = Proc{}
	refused(t, r, h1)
}

func TestStopRefusesADeadWrapper(t *testing.T) {
	r := newStopRig(t)
	r.f.record(t, h3, "work", epoch, func(x *registry.Record) { x.WrapperPID = deadPID(t); x.PID = 4242 })
	refused(t, r, h3)
}

func TestStopRefusesASessionWhoseClaudeHasNotStarted(t *testing.T) {
	r := newStopRig(t)
	r.f.record(t, h3, "work", epoch, func(x *registry.Record) { x.PID = 0 })
	refused(t, r, h3)
}

func TestStopRefusesAPidThatStartedBeforeTheSession(t *testing.T) {
	r := newStopRig(t)
	r.proc.Start = epoch.Add(-time.Hour)
	refused(t, r, h1)
}

func TestStopRefusesAPidThatIsNotAChildOfTheWrapper(t *testing.T) {
	r := newStopRig(t)
	r.proc.PPID = r.rec.WrapperPID + 1
	refused(t, r, h1)
}

func TestStopRefusesAProcessThatIsNotClaude(t *testing.T) {
	r := newStopRig(t)
	r.proc.Command = "/usr/bin/vim notes.txt"
	refused(t, r, h1)
}

func TestStopDoesNotSignalWhenTheProcessCannotBeLookedAt(t *testing.T) {
	r := newStopRig(t)
	r.probe = errors.New("ps is not here")
	if err := r.s.Stop(context.Background(), "work", h1); err == nil || len(r.signed) != 0 {
		t.Errorf("err=%v signed=%v", err, r.signed)
	}
}

func TestStopSurfacesASignalFailure(t *testing.T) {
	r := newStopRig(t)
	r.s.Signal = func(int) error { return errors.New("operation not permitted") }
	if err := r.s.Stop(context.Background(), "work", h1); err == nil {
		t.Error("a failed signal was reported as a stop")
	}
}

func TestIsClaudeCommand(t *testing.T) {
	for cmd, want := range map[string]bool{
		"claude":                            true,
		"/opt/homebrew/bin/claude --resume": true,
		"node /usr/lib/claude/cli.js":       true,
		"/Users/x/.local/bin/claude.exe -p": true,
		"/usr/bin/vim claude-notes.txt":     false,
		"/bin/sh -c echo claude":            false,
		"":                                  false,
	} {
		if got := isClaudeCommand(cmd); got != want {
			t.Errorf("isClaudeCommand(%q) = %v, want %v", cmd, got, want)
		}
	}
}

func TestStopRefusesARecordWithNoWrapperPid(t *testing.T) {
	r := newStopRig(t)
	r.f.record(t, h3, "work", epoch, func(x *registry.Record) { x.WrapperPID = 0; x.PID = 4242 })
	refused(t, r, h3)
}

func TestStopAllowsAFewSecondsOfClockSlackButNotAnEarlierProcess(t *testing.T) {
	r := newStopRig(t)
	r.proc.Start = epoch.Add(-time.Second)
	if err := r.s.Stop(context.Background(), "work", h1); err != nil {
		t.Errorf("a start a second before the record was refused: %v", err)
	}
	r.signed = nil
	r.proc.Start = epoch.Add(-5 * time.Second)
	refused(t, r, h1)
}

func TestIsClaudeCommandWithSpacesInThePath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my apps")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "claude")
	if err := os.WriteFile(bin, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "vim")
	if err := os.WriteFile(other, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	for cmd, want := range map[string]bool{
		bin + " --resume abc": true,
		other + " claude":     false,
		"node /opt/x/node_modules/@anthropic/claude/cli.js -p": true,
		"node /opt/x/other/cli.js":                             false,
		"node /opt/claude-tools/script.js":                     false,
		"bun /opt/bin/claude":                                  true,
	} {
		if got := isClaudeCommand(cmd); got != want {
			t.Errorf("isClaudeCommand(%q) = %v, want %v", cmd, got, want)
		}
	}
}
