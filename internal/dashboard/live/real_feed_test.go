package live

import (
	"context"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/dashboard"
	"github.com/dhitalkamal/clalt/internal/feed"
)

func feedFiles(t *testing.T, now time.Time) (FeedFiles, files) {
	f := newFiles(t)
	return FeedFiles{Store: f.fd, Now: func() time.Time { return now }}, f
}

func TestFeedFilesGiveBothWindowsAndTheirAge(t *testing.T) {
	ff, f := feedFiles(t, epoch)
	five := &feed.Window{UsedPct: 40, ResetsAt: epoch.Add(2 * time.Hour)}
	seven := &feed.Window{UsedPct: 12.6, ResetsAt: epoch.Add(72 * time.Hour)}
	if err := f.fd.MergeLimits("work", five, seven, epoch.Add(-10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, ok, err := ff.Limits(context.Background(), "work")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if got.Five.Percent != 40 || got.Seven.Percent != 13 || got.Five.Unknown || got.Seven.Unknown {
		t.Errorf("limits = %+v", got)
	}
	if !got.ObservedAt.Equal(epoch.Add(-10 * time.Minute)) {
		t.Errorf("observed = %v", got.ObservedAt)
	}
}

func TestAWindowTheFeedLacksIsUnknownNotZero(t *testing.T) {
	ff, f := feedFiles(t, epoch)
	if err := f.fd.MergeLimits("work", &feed.Window{UsedPct: 5, ResetsAt: epoch.Add(time.Hour)}, nil, epoch); err != nil {
		t.Fatal(err)
	}
	got, ok, _ := ff.Limits(context.Background(), "work")
	if !ok || got.Seven.Unknown != true || got.Five.Unknown {
		t.Fatalf("limits = %+v ok=%v", got, ok)
	}
	if d := mapLimits(got, ok, nil, epoch); !d.Seven.Unknown || d.Five.Unknown {
		t.Errorf("mapped = %+v", d)
	}
}

func TestNoFileIsNoReading(t *testing.T) {
	ff, _ := feedFiles(t, epoch)
	if _, ok, err := ff.Limits(context.Background(), "work"); ok || err != nil {
		t.Errorf("ok=%v err=%v", ok, err)
	}
	if _, ok, err := ff.Limits(context.Background(), "../x"); ok || err != nil {
		t.Errorf("bad id: ok=%v err=%v", ok, err)
	}
}

func TestAReadingWhoseWindowPassedMeansItReset(t *testing.T) {
	observed := epoch
	ff, f := feedFiles(t, observed.Add(6*time.Hour))
	if err := f.fd.MergeLimits("work", &feed.Window{UsedPct: 90, ResetsAt: observed.Add(time.Hour)},
		&feed.Window{UsedPct: 30, ResetsAt: observed.Add(48 * time.Hour)}, observed); err != nil {
		t.Fatal(err)
	}
	l, ok, _ := ff.Limits(context.Background(), "work")
	d := mapLimits(l, ok, nil, observed.Add(6*time.Hour))
	if d.Five != (dashboard.Window{}) {
		t.Errorf("5h = %+v, want a reset window", d.Five)
	}
	if d.Seven.Percent != 30 || !d.ReadAt.Equal(observed) {
		t.Errorf("7d = %+v readAt = %v", d.Seven, d.ReadAt)
	}
}

func TestInstalledIsWhatTheFeedHasSeenOfTheAccount(t *testing.T) {
	ff, f := feedFiles(t, epoch)
	if ff.Installed("work") {
		t.Error("installed with an empty feed")
	}
	f.state(t, h1, "work", epoch, func(s *feed.SessionState) {})
	if !ff.Installed("work") || ff.Installed("other") {
		t.Error("installed should follow the sessions the feed has")
	}
}

func TestFeedFilesKeepNoReadingHistory(t *testing.T) {
	ff, _ := feedFiles(t, epoch)
	if r, err := ff.Readings(context.Background(), "work", epoch); r != nil || err != nil {
		t.Errorf("readings = %v %v", r, err)
	}
}
