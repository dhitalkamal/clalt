package live

import (
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
)

func TestRegistryAndFeedFilesReachTheSnapshot(t *testing.T) {
	r := newRig(t)
	id := r.signedIn("work", "w@example.com")
	f := newFiles(t)
	r.cfg.Sessions = &RegistrySessions{Reg: f.reg, Feed: f.fd, Now: r.clock.now}
	r.cfg.Feed = FeedFiles{Store: f.fd, Now: r.clock.now}
	f.record(t, h1, id, epoch.Add(-time.Hour), nil)
	f.state(t, h1, id, epoch, func(s *feed.SessionState) { s.State, s.Model = feed.StateBusy, "claude-opus-5-5" })
	if err := f.fd.MergeLimits(id, &feed.Window{UsedPct: 30, ResetsAt: epoch.Add(3 * time.Hour)}, nil, epoch.Add(-5*time.Minute)); err != nil {
		t.Fatal(err)
	}

	a := byLabel(r.load(r.adapter()), "work")

	if len(a.Sessions) != 1 || a.Sessions[0].State != "working" || a.Sessions[0].Model != "Opus 5.5" {
		t.Errorf("sessions = %+v", a.Sessions)
	}
	if a.Limits.Five.Percent != 30 || a.Limits.Seven.Unknown != true || !a.Profile.FeedInstalled {
		t.Errorf("limits = %+v installed = %v", a.Limits, a.Profile.FeedInstalled)
	}
	if want := epoch.Add(-5 * time.Minute); !a.Limits.ReadAt.Equal(want) {
		t.Errorf("read at %v, want %v", a.Limits.ReadAt, want)
	}
}
