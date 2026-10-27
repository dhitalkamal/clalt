package notify

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/runcmd"
)

func limitEvent() LimitEvent {
	return LimitEvent{AccountID: "work", Account: "WORK", Minutes: 20, WindowEnds: noon.Add(2 * time.Hour), Alt: "PERSONAL", AltPct: 12}
}

func TestLimitSoonSaysWhichAccountAndWhereToMove(t *testing.T) {
	a, f := newAlerts(t)
	out, err := a.LimitSoon(context.Background(), limitEvent())
	if out != Sent || err != nil {
		t.Fatalf("out %v err %v", out, err)
	}
	args := f.Calls()[0].Args
	if got := args[len(args)-1]; got != "WORK hits its 5h limit in about 20 min. Move sessions to PERSONAL (12%)." {
		t.Errorf("text = %q", got)
	}
}

func TestLimitSoonWithNoOtherAccountSaysNothingAboutMoving(t *testing.T) {
	a, f := newAlerts(t)
	e := limitEvent()
	e.Alt = ""
	a.LimitSoon(context.Background(), e)
	args := f.Calls()[0].Args
	if got := args[len(args)-1]; strings.Contains(got, "Move") || !strings.HasPrefix(got, "WORK hits its 5h limit") {
		t.Errorf("text = %q", got)
	}
}

func TestLimitSoonIsSentOncePerWindow(t *testing.T) {
	a, f := newAlerts(t)
	ctx := context.Background()
	if out, _ := a.LimitSoon(ctx, limitEvent()); out != Sent {
		t.Fatalf("first = %v", out)
	}
	for i := 0; i < 3; i++ {
		if out, _ := a.LimitSoon(ctx, limitEvent()); out != AlreadyNotified {
			t.Fatalf("repeat = %v", out)
		}
	}
	if n := len(f.Calls()); n != 1 {
		t.Errorf("%d notifications, want 1", n)
	}
	// the next window of the same account is a new warning
	next := limitEvent()
	next.WindowEnds = next.WindowEnds.Add(5 * time.Hour)
	if out, _ := a.LimitSoon(ctx, next); out != Sent {
		t.Errorf("next window = %v", out)
	}
	// and another account in the same window is its own
	other := limitEvent()
	other.AccountID = "side"
	if out, _ := a.LimitSoon(ctx, other); out != Sent {
		t.Errorf("other account = %v", out)
	}
}

func TestLimitSoonForgetsOldMarkers(t *testing.T) {
	a, _ := newAlerts(t)
	old := limitEvent()
	old.WindowEnds = noon.Add(-48 * time.Hour)
	a.LimitSoon(context.Background(), old)
	dir := filepath.Join(a.Dir, "limits")
	files, _ := os.ReadDir(dir)
	for _, fi := range files {
		_ = os.Chtimes(filepath.Join(dir, fi.Name()), noon.Add(-48*time.Hour), noon.Add(-48*time.Hour))
	}
	a.LimitSoon(context.Background(), limitEvent())
	files, _ = os.ReadDir(dir)
	if len(files) != 1 {
		t.Errorf("%d markers left, want only the current one", len(files))
	}
}

func TestLimitSoonRespectsTheSettingAndAMissingNotifier(t *testing.T) {
	a, f := newAlerts(t)
	if err := os.WriteFile(filepath.Join(a.Dir, "settings.json"), []byte(`{"enabled":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, _ := a.LimitSoon(context.Background(), limitEvent()); out != Disabled || len(f.Calls()) != 0 {
		t.Errorf("disabled: out %v calls %d", out, len(f.Calls()))
	}
	b := &Alerts{Dir: t.TempDir(), Now: func() time.Time { return noon }}
	if out, _ := b.LimitSoon(context.Background(), limitEvent()); out != NoNotifier {
		t.Errorf("no notifier: out %v", out)
	}
}

func TestLimitSoonTriesAgainAfterAFailedSend(t *testing.T) {
	f := runcmd.NewFake()
	n, _ := System(f, "darwin")
	a := &Alerts{Notifier: &failingOnce{n: n}, Dir: t.TempDir(), Now: func() time.Time { return noon }}
	if _, err := a.LimitSoon(context.Background(), limitEvent()); err == nil {
		t.Fatal("want the send error")
	}
	if out, err := a.LimitSoon(context.Background(), limitEvent()); out != Sent || err != nil {
		t.Errorf("retry = %v, %v", out, err)
	}
}

type failingOnce struct {
	n      Notifier
	failed bool
}

func (f *failingOnce) Notify(ctx context.Context, n Note) error {
	if !f.failed {
		f.failed = true
		return errors.New("no display")
	}
	return f.n.Notify(ctx, n)
}

func TestLimitSoonRefusesABadAccountId(t *testing.T) {
	a, _ := newAlerts(t)
	e := limitEvent()
	e.AccountID = "../x"
	if _, err := a.LimitSoon(context.Background(), e); !errors.Is(err, ErrBadHandle) {
		t.Errorf("err = %v", err)
	}
}
