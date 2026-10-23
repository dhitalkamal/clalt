package notify

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/runcmd"
)

var noon = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

func newAlerts(t *testing.T) (*Alerts, *runcmd.Fake) {
	t.Helper()
	f := runcmd.NewFake()
	n, _ := System(f, "darwin")
	return &Alerts{Notifier: n, Dir: t.TempDir(), Now: func() time.Time { return noon }}, f
}

func marker(a *Alerts, h string) string { return filepath.Join(a.Dir, "waiting", h) }

func TestNeedsYouSendsTitleAndText(t *testing.T) {
	a, f := newAlerts(t)
	out, err := a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "work", Reason: "permission needed", Cwd: "/src/clalt/"})
	if out != Sent || err != nil {
		t.Fatalf("out %v err %v", out, err)
	}
	args := f.Calls()[0].Args
	if args[len(args)-2] != "work needs you" || args[len(args)-1] != "permission needed (clalt)" {
		t.Errorf("args = %q", args)
	}
	if fi, err := os.Stat(marker(a, "h1")); err != nil || fi.Size() != 0 {
		t.Errorf("marker: %v %v", fi, err)
	}
}

func TestNeedsYouDefaultReasonAndNoCwd(t *testing.T) {
	a, f := newAlerts(t)
	a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "work"})
	args := f.Calls()[0].Args
	if args[len(args)-1] != "waiting for you" {
		t.Errorf("text = %q", args[len(args)-1])
	}
}

func TestSecondNeedsYouIsAlreadyNotified(t *testing.T) {
	a, f := newAlerts(t)
	a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "w"})
	out, err := a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "w"})
	if out != AlreadyNotified || err != nil || len(f.Calls()) != 1 {
		t.Errorf("out %v err %v calls %d", out, err, len(f.Calls()))
	}
}

func TestEndWaitReopensTheLimitAndToleratesMissing(t *testing.T) {
	a, f := newAlerts(t)
	if err := a.EndWait("never"); err != nil {
		t.Errorf("missing: %v", err)
	}
	a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "w"})
	if err := a.EndWait("h1"); err != nil {
		t.Fatal(err)
	}
	out, _ := a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "w"})
	if out != Sent || len(f.Calls()) != 2 {
		t.Errorf("out %v calls %d", out, len(f.Calls()))
	}
}

func TestOldMarkerCountsAsGone(t *testing.T) {
	a, f := newAlerts(t)
	os.MkdirAll(filepath.Dir(marker(a, "h1")), 0o700)
	os.WriteFile(marker(a, "h1"), nil, 0o600)
	old := noon.Add(-6*time.Hour - time.Minute)
	os.Chtimes(marker(a, "h1"), old, old)
	out, _ := a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "w"})
	if out != Sent || len(f.Calls()) != 1 {
		t.Errorf("out %v calls %d", out, len(f.Calls()))
	}
	fresh := noon.Add(-5 * time.Hour)
	os.Chtimes(marker(a, "h1"), fresh, fresh)
	if out, _ := a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "w"}); out != AlreadyNotified {
		t.Errorf("fresh marker: out %v", out)
	}
}

func TestFailedSendRemovesTheMarker(t *testing.T) {
	a, f := newAlerts(t)
	f.On("osascript").Fail(1, "boom")
	out, err := a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "w"})
	if err == nil || out == Sent {
		t.Errorf("out %v err %v", out, err)
	}
	if _, serr := os.Stat(marker(a, "h1")); !errors.Is(serr, os.ErrNotExist) {
		t.Errorf("marker left: %v", serr)
	}
}

func TestDisabledMakesNoMarker(t *testing.T) {
	a, f := newAlerts(t)
	SetEnabled(a.Dir, false)
	out, err := a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "w"})
	if out != Disabled || err != nil || len(f.Calls()) != 0 {
		t.Errorf("out %v err %v calls %d", out, err, len(f.Calls()))
	}
	if _, serr := os.Stat(marker(a, "h1")); !errors.Is(serr, os.ErrNotExist) {
		t.Error("marker made while disabled")
	}
}

func TestNoNotifierMakesNoMarker(t *testing.T) {
	a, _ := newAlerts(t)
	a.Notifier = nil
	out, err := a.NeedsYou(context.Background(), Event{Handle: "h1", Account: "w"})
	if out != NoNotifier || err != nil {
		t.Errorf("out %v err %v", out, err)
	}
	if _, serr := os.Stat(marker(a, "h1")); !errors.Is(serr, os.ErrNotExist) {
		t.Error("marker made without a notifier")
	}
}

func TestBadHandlesTouchNothing(t *testing.T) {
	for _, h := range []string{"", ".hidden", "a/b", "..", "a b", strings.Repeat("x", 65), "a\x00b", "h\u00e9"} {
		a, f := newAlerts(t)
		if _, err := a.NeedsYou(context.Background(), Event{Handle: h, Account: "w"}); !errors.Is(err, ErrBadHandle) {
			t.Errorf("NeedsYou(%q) err = %v", h, err)
		}
		if err := a.EndWait(h); !errors.Is(err, ErrBadHandle) {
			t.Errorf("EndWait(%q) err = %v", h, err)
		}
		if _, err := os.Stat(a.Dir + "/waiting"); err == nil || len(f.Calls()) != 0 {
			t.Errorf("handle %q touched something", h)
		}
	}
	a, _ := newAlerts(t)
	if _, err := a.NeedsYou(context.Background(), Event{Handle: strings.Repeat("A", 64) + "", Account: "w"}); err != nil {
		t.Errorf("64 chars refused: %v", err)
	}
	if _, err := a.NeedsYou(context.Background(), Event{Handle: "a.b-c_9", Account: "w"}); err != nil {
		t.Errorf("good handle refused: %v", err)
	}
}

func TestDefaultUsesClaltDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/x/cfg")
	a := Default()
	if a.Dir != filepath.Join("/x/cfg", "clalt", "notify") || a.Now == nil {
		t.Errorf("Dir %q", a.Dir)
	}
	_ = reflect.DeepEqual
}
