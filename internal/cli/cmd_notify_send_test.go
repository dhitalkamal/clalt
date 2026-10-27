package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/runcmd"
)

func TestNotifyTestSendsOneSampleThroughTheFake(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()

	code := e.run("notify", "test")

	want := []runcmd.Call{{Name: "osascript", Args: []string{
		"-e", "on run argv",
		"-e", "display notification (item 3 of argv) with title (item 1 of argv) subtitle (item 2 of argv)",
		"-e", "end run",
		"clalt", "clalt", "clalt notifications work"}}}
	if code != 0 || !reflect.DeepEqual(f.run.Calls(), want) {
		t.Errorf("code %d, calls %v", code, f.run.Calls())
	}
}

func TestNotifyTestIgnoresTheSetting(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()
	e.run("notify", "off")

	code := e.run("notify", "test")

	if code != 0 || len(f.run.Calls()) != 1 {
		t.Errorf("code %d, calls %v", code, f.run.Calls())
	}
}

func TestNotifyTestWithoutANotifierErrors(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()
	e.app.deps().goos = "linux"
	f.run.Without("notify-send")

	code := e.run("notify", "test")

	if code != 1 || !strings.Contains(e.errOut.String(), "no notifier is available") {
		t.Errorf("code %d, err %q", code, e.errOut.String())
	}
	if len(f.run.Calls()) != 0 {
		t.Errorf("ran %v", f.run.Calls())
	}
}

func TestNotifyTestReportsAFailedSend(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()
	f.run.On("osascript").Fail(1, "not allowed")

	code := e.run("notify", "test")

	if code != 1 || !strings.Contains(e.errOut.String(), "not allowed") {
		t.Errorf("code %d, err %q", code, e.errOut.String())
	}
}
