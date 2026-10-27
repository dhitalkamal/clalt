package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/notify"
)

func notifyDir(e *testEnv) string { return filepath.Join(e.home, ".config", "clalt", "notify") }

func TestNotifyStatusIsTheDefaultAndRunsNothing(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()

	code := e.run("notify")

	out := e.out.String()
	if code != 0 || !strings.Contains(out, "needs-you notifications are on") || !strings.Contains(out, "macOS notification (osascript)") {
		t.Errorf("code %d, out %q", code, out)
	}
	if len(f.run.Calls()) != 0 {
		t.Errorf("status ran %v", f.run.Calls())
	}
}

func TestNotifyOffThenOnWritesTheSetting(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()

	code := e.run("notify", "off")
	if code != 0 || !strings.Contains(e.out.String(), "needs-you notifications are off") {
		t.Errorf("off: code %d, out %q", code, e.out.String())
	}
	if on, _ := notify.Enabled(notifyDir(e)); on {
		t.Error("the setting is still on")
	}
	e.reset()
	e.run("notify", "status")
	if !strings.Contains(e.out.String(), "are off") {
		t.Errorf("status after off: %q", e.out.String())
	}
	e.reset()
	code = e.run("notify", "on")
	if code != 0 || !strings.Contains(e.out.String(), "needs-you notifications are on") {
		t.Errorf("on: code %d, out %q", code, e.out.String())
	}
	if on, _ := notify.Enabled(notifyDir(e)); !on {
		t.Error("the setting is still off")
	}
	if len(f.run.Calls()) != 0 {
		t.Errorf("ran %v", f.run.Calls())
	}
}

func TestNotifyStatusNamesNotifySendOrNothing(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()
	e.app.deps().goos = "linux"

	e.run("notify", "status")
	if !strings.Contains(e.out.String(), "notify-send") {
		t.Errorf("linux: %q", e.out.String())
	}

	e.reset()
	f.run.Without("notify-send")
	e.run("notify", "status")
	if !strings.Contains(e.out.String(), "nothing available on this machine") {
		t.Errorf("no notify-send: %q", e.out.String())
	}
}

func TestNotifyStatusSaysWhenTheSettingsAreUnreadable(t *testing.T) {
	e := newEnv(t)
	e.useFakeDeps()
	os.MkdirAll(notifyDir(e), 0o700)
	os.WriteFile(filepath.Join(notifyDir(e), "settings.json"), []byte("{nope"), 0o600)

	code := e.run("notify", "status")

	out := e.out.String()
	if code != 0 || !strings.Contains(out, "are on") || !strings.Contains(out, "could not be read") || !strings.Contains(out, "default (on)") {
		t.Errorf("code %d, out %q", code, out)
	}
}

func TestNotifyRejectsOtherWordsAndExtraArgs(t *testing.T) {
	for _, args := range [][]string{{"notify", "loud"}, {"notify", "on", "now"}, {"notify", "status", "x"}} {
		e := newEnv(t)
		f := e.useFakeDeps()

		code := e.run(args...)

		if code != 2 || !strings.Contains(e.errOut.String(), "clalt notify [on|off|status|test]") {
			t.Errorf("%v: code %d, err %q", args, code, e.errOut.String())
		}
		if len(f.run.Calls()) != 0 {
			t.Errorf("%v ran %v", args, f.run.Calls())
		}
	}
}
