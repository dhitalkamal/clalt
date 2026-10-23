package live

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
)

// deadPID is the pid of a process that has run and been collected
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

// files is a registry and a feed in temp folders
type files struct {
	reg registry.Registry
	fd  feed.Store
}

func newFiles(t *testing.T) files {
	t.Helper()
	d := t.TempDir()
	return files{reg: registry.Registry{Dir: d + "/sessions"}, fd: feed.Store{Dir: d + "/feed"}}
}

// record writes a registry record whose wrapper is this test process
func (f files) record(t *testing.T, handle, account string, started time.Time, mod func(*registry.Record)) registry.Record {
	t.Helper()
	rec := registry.Record{Handle: handle, Account: account, WrapperPID: os.Getpid(), PID: os.Getpid(), Cwd: "/work/app", StartedAt: started}
	if mod != nil {
		mod(&rec)
	}
	if err := f.reg.Create(rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func (f files) state(t *testing.T, handle, account string, now time.Time, fn func(*feed.SessionState)) {
	t.Helper()
	err := f.fd.UpdateSession(handle, account, now, func(s *feed.SessionState) bool { fn(s); return true })
	if err != nil {
		t.Fatal(err)
	}
}

const (
	h1 = "0123456789abcdef"
	h2 = "fedcba9876543210"
	h3 = "aaaaaaaaaaaaaaaa"
)
