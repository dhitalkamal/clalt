//go:build !windows

package claudecli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// pgidOf is the process group that the fake found itself in on its last call of
// one command.
func (f *fake) pgidOf(name string) int {
	f.t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.dir, "pgid."+name))
	if err != nil {
		f.t.Fatalf("the fake never recorded its group for %s: %v", name, err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		f.t.Fatalf("pgid.%s holds %q", name, raw)
	}
	return n
}

// a Ctrl-C at the terminal goes to every process of the foreground group. real
// claude has no SIGINT handler, so a status check that gets one in the middle of
// a token refresh can sign the folder out. the calls clalt makes on its own start
// in a group of their own, so that the interrupt ends clalt and lets them finish.
func TestCallsThatNeedNoTerminalStartInAGroupOfTheirOwn(t *testing.T) {
	f := newFake(t)
	f.say("status", 0, signedInJSON)
	f.say("logout", 0, "")
	dir := t.TempDir()

	if _, err := Status(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	f.say("status", 1, signedOutJSON)
	if err := Logout(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	if _, err := Version(context.Background()); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"status", "logout", "version"} {
		if got := f.pgidOf(name); got == syscall.Getpgrp() || got <= 0 {
			t.Errorf("%s ran in group %d, clalt's own is %d: the terminal's interrupt would reach it", name, got, syscall.Getpgrp())
		}
	}
}

// the sign-in and the session read the terminal, and a process outside the
// foreground group is stopped when it tries to
func TestTheSignInAndTheSessionStayInTheForegroundGroup(t *testing.T) {
	f := newFake(t)
	dir := t.TempDir()

	login, err := Login(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := login.Run(); err != nil {
		t.Fatal(err)
	}
	if got := f.pgidOf("login"); got != syscall.Getpgrp() {
		t.Errorf("the sign-in ran in group %d, clalt's own is %d", got, syscall.Getpgrp())
	}

	session, err := Session(context.Background(), dir, []string{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Run(); err != nil {
		t.Fatal(err)
	}
	if got := f.pgidOf("version"); got != syscall.Getpgrp() {
		t.Errorf("the session ran in group %d, clalt's own is %d", got, syscall.Getpgrp())
	}
}

// the test below runs this in a process of its own, which makes a status call.
// TestMain points that process at the guard, so the fake is handed over by name.
func TestHelperThatMakesAStatusCall(t *testing.T) {
	folder := os.Getenv("CLALT_TEST_STATUS_FOLDER")
	if folder == "" {
		t.Skip("only runs as the helper of another test")
	}
	t.Setenv("CLALT_CLAUDE_BIN", os.Getenv("CLALT_TEST_FAKE"))
	_, _ = Status(context.Background(), folder)
}

// the terminal's Ctrl-C is a SIGINT to the whole foreground group, here the group
// of a stand-in for clalt. it ends clalt, and the status call it was waiting for is
// left to finish on its own.
func TestAnInterruptAtTheTerminalEndsClaltAndLeavesAStatusCallAlone(t *testing.T) {
	f := newFake(t)
	f.hangOn("status", "settle")
	helper := exec.Command(os.Args[0], "-test.run=^TestHelperThatMakesAStatusCall$")
	helper.Env = append(os.Environ(), "CLALT_TEST_STATUS_FOLDER="+t.TempDir(), "CLALT_TEST_FAKE="+os.Getenv("CLALT_CLAUDE_BIN"))
	helper.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = helper.Process.Kill() })
	waitFor(t, func() bool { return f.has("ready") })

	if err := syscall.Kill(-helper.Process.Pid, syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	_ = helper.Wait()

	waitFor(t, func() bool { return f.has("interrupted") || f.has("finished") })
	if f.has("interrupted") {
		t.Error("the status call got the interrupt that was meant for clalt")
	}
	if !f.has("finished") {
		t.Error("the status call did not get to finish")
	}
}

// waitFor polls until cond holds, for a few seconds.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("gave up waiting")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
