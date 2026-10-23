package cli

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
)

// the dashboard reads the registry and feed that clalt run and the hooks write
func TestTheLiveAdapterShowsSessionsAndLimitsFromTheFiles(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	acc, err := e.app.Store.List()
	if err != nil || len(acc) == 0 {
		t.Fatalf("accounts %v err %v", acc, err)
	}
	id, now := acc[0].Name, time.Now()
	const handle = "0123456789abcdef"
	reg := registry.Registry{Dir: config.SessionsDir()}
	rec := registry.Record{Handle: handle, Account: id, WrapperPID: os.Getpid(), PID: os.Getpid(), Cwd: e.home, StartedAt: now.Add(-time.Minute)}
	if err := reg.Create(rec); err != nil {
		t.Fatal(err)
	}
	fd := feed.Store{Dir: config.FeedDir()}
	if err := fd.UpdateSession(handle, id, now, func(s *feed.SessionState) bool { s.State = feed.StateBusy; return true }); err != nil {
		t.Fatal(err)
	}
	if err := fd.MergeLimits(id, &feed.Window{UsedPct: 42, ResetsAt: now.Add(time.Hour)}, nil, now); err != nil {
		t.Fatal(err)
	}

	snap, err := e.app.liveAdapter().Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	got := snap.Accounts[0]
	if len(got.Sessions) != 1 || got.Sessions[0].State != "working" || got.Limits.Five.Percent != 42 || !got.Limits.Seven.Unknown {
		t.Errorf("account = %+v", got)
	}
}
