package cli

import (
	"context"
	"errors"
	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/fslock"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/notify"
	"github.com/dhitalkamal/clalt/internal/runcmd"
)

const permissionPrompt = `{"session_id":"s","cwd":"/home/me/proj","notification_type":"permission_prompt","message":"needs approval"}`

// osascriptCalls counts the fake notifications shown so far.
func osascriptCalls(f *fakeDeps) int {
	n := 0
	for _, c := range f.run.Calls() {
		if c.Name == "osascript" {
			n++
		}
	}
	return n
}

func TestHookNotifiesOnceForAPermissionPrompt(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()
	liveHandle(t)
	e.hook("UserPromptSubmit", `{}`)

	e.hook("Notification", permissionPrompt)

	calls := f.run.Calls()
	if len(calls) != 1 || calls[0].Name != "osascript" {
		t.Fatalf("calls %v", calls)
	}
	text := strings.Join(calls[0].Args, " ")
	if !strings.Contains(text, "work needs you") || !strings.Contains(text, "(proj)") {
		t.Errorf("the notification lacks the account or the folder: %q", text)
	}
	if strings.Contains(text, "needs approval") {
		t.Errorf("claude's message went into the notification: %q", text)
	}
	if e.out.Len() != 0 || e.errOut.Len() != 0 {
		t.Errorf("out %q err %q", e.out.String(), e.errOut.String())
	}
}

func TestHookDoesNotNotifyTwiceInTheSameWait(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()
	liveHandle(t)
	e.hook("UserPromptSubmit", `{}`)

	e.hook("Notification", permissionPrompt)
	e.hook("Notification", permissionPrompt)

	if n := osascriptCalls(f); n != 1 {
		t.Errorf("%d notifications", n)
	}
}

func TestHookNotifiesAgainAfterBusyThenNeedsYou(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()
	liveHandle(t)
	e.hook("UserPromptSubmit", `{}`)
	e.hook("Notification", permissionPrompt)

	// the prompt was answered and claude went on, then it waits again
	e.hook("UserPromptSubmit", `{}`)
	e.hook("Notification", `{"notification_type":"elicitation_dialog"}`)

	if n := osascriptCalls(f); n != 2 {
		t.Errorf("%d notifications, want 2", n)
	}
}

func TestHookNotifiesAgainAfterAStop(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()
	liveHandle(t)
	e.hook("UserPromptSubmit", `{}`)
	e.hook("Notification", permissionPrompt)
	e.hook("Stop", `{}`)
	e.hook("UserPromptSubmit", `{}`)

	e.hook("Notification", permissionPrompt)

	if n := osascriptCalls(f); n != 2 {
		t.Errorf("%d notifications, want 2", n)
	}
}

func TestHookStaysQuietWhenNotificationsAreOff(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()
	liveHandle(t)
	if err := notify.SetEnabled(notifyDir(e), false); err != nil {
		t.Fatal(err)
	}
	e.hook("UserPromptSubmit", `{}`)

	e.hook("Notification", permissionPrompt)

	if n := osascriptCalls(f); n != 0 {
		t.Errorf("%d notifications with the setting off", n)
	}
}

func TestHookNotifiesForIdlePromptOnlyWithItsSetting(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()
	liveHandle(t)
	e.hook("Stop", `{}`)

	e.hook("Notification", `{"notification_type":"idle_prompt"}`)
	if n := osascriptCalls(f); n != 0 {
		t.Fatalf("%d notifications by default for an idle prompt", n)
	}

	if err := notify.SetIdleEnabled(notifyDir(e), true); err != nil {
		t.Fatal(err)
	}
	e.hook("Notification", `{"notification_type":"idle_prompt"}`)
	if n := osascriptCalls(f); n != 1 {
		t.Errorf("%d notifications with the idle setting on", n)
	}
}

func TestHookIdleSettingDoesNotTurnOnNeedsYouWhenOff(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()
	liveHandle(t)
	notify.SetEnabled(notifyDir(e), false)
	notify.SetIdleEnabled(notifyDir(e), true)

	e.hook("Notification", `{"notification_type":"idle_prompt"}`)

	if n := osascriptCalls(f); n != 0 {
		t.Errorf("%d notifications with the main setting off", n)
	}
}

func TestHookStaysQuietAndExitsZeroWhenTheNotifierFails(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()
	f.run.On("osascript").Err(errors.New("boom"))
	liveHandle(t)
	e.hook("UserPromptSubmit", `{}`)

	code := e.hook("Notification", permissionPrompt)

	if code != 0 || e.out.Len() != 0 || e.errOut.Len() != 0 {
		t.Errorf("exit %d out %q err %q", code, e.out.String(), e.errOut.String())
	}
}

// hangRunner never finishes until its context ends.
type hangRunner struct{ runcmd.Runner }

func (hangRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestHookDoesNotWaitLongForAHangingNotifier(t *testing.T) {
	e := newEnv(t)
	e.useFakeDeps()
	e.app.sdeps.runner = hangRunner{}
	old := notifyTimeout
	notifyTimeout = 100 * time.Millisecond
	t.Cleanup(func() { notifyTimeout = old })
	liveHandle(t)
	e.hook("UserPromptSubmit", `{}`)

	start := time.Now()
	code := e.hook("Notification", permissionPrompt)

	if code != 0 || e.out.Len() != 0 || e.errOut.Len() != 0 || time.Since(start) > 2*time.Second {
		t.Errorf("exit %d out %q err %q after %v", code, e.out.String(), e.errOut.String(), time.Since(start))
	}
}

func TestHookDoesNotNotifyForAPromptThatWasAlreadyAnswered(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()
	liveHandle(t)
	e.hook("UserPromptSubmit", `{}`)
	e.hook("PostToolUse", `{"tool_name":"Bash"}`)

	e.hook("Notification", permissionPrompt)

	if n := osascriptCalls(f); n != 0 {
		t.Errorf("%d notifications for a late prompt", n)
	}
}

func TestNotifyIdleSwitchKeepsTheMainSetting(t *testing.T) {
	e := newEnv(t)
	e.useFakeDeps()
	e.run("notify", "off")

	if code := e.run("notify", "idle", "on"); code != 0 {
		t.Fatalf("exit %d", code)
	}

	dir := notifyDir(e)
	if on, _ := notify.Enabled(dir); on {
		t.Error("the main setting was turned back on")
	}
	if on, _ := notify.IdleEnabled(dir); !on {
		t.Error("the idle setting was not saved")
	}
}

func TestHookWithContendedLocksAndAHangingNotifierStaysInItsBudget(t *testing.T) {
	e := newEnv(t)
	e.useFakeDeps()
	e.app.sdeps.runner = hangRunner{}
	oldBudget, oldNotify := hookBudget, notifyTimeout
	hookBudget, notifyTimeout = 600*time.Millisecond, 400*time.Millisecond
	t.Cleanup(func() { hookBudget, notifyTimeout = oldBudget, oldNotify })
	liveHandle(t)
	e.hook("UserPromptSubmit", `{}`)
	// the registry lock is held, so recording the claude session id waits
	l, err := fslock.Acquire(filepath.Join(config.SessionsDir(), ".lock"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()

	start := time.Now()
	code := e.hook("Notification", permissionPrompt)

	if code != 0 || e.out.Len() != 0 || e.errOut.Len() != 0 {
		t.Errorf("exit %d out %q err %q", code, e.out.String(), e.errOut.String())
	}
	if d := time.Since(start); d > hookBudget+300*time.Millisecond {
		t.Errorf("the hook took %v with a budget of %v", d, hookBudget)
	}
}

func TestStopEndsTheWaitSoAnIdlePromptNotifies(t *testing.T) {
	e := newEnv(t)
	f := e.useFakeDeps()
	liveHandle(t)
	notify.SetIdleEnabled(notifyDir(e), true)
	e.hook("UserPromptSubmit", `{}`)
	e.hook("Notification", permissionPrompt)

	e.hook("Stop", `{}`)
	e.hook("Notification", `{"notification_type":"idle_prompt"}`)

	if n := osascriptCalls(f); n != 2 {
		t.Errorf("%d notifications, want 2", n)
	}
}
