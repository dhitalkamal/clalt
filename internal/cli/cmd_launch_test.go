package cli

import (
	"github.com/dhitalkamal/clalt/internal/terminal"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/runcmd"
)

// launchEnv is a signed-in account "work", fake deps inside tmux and a folder to
// open the tab in.
func launchEnv(t *testing.T) (*testEnv, *fakeDeps, string) {
	t.Helper()
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	f := e.useFakeDeps()
	f.env["TMUX"] = "/tmp/tmux-1/default,1,0"
	f.run.On("tmux", "new-window").Output("main:3\n")
	dir := filepath.Join(e.home, "proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return e, f, dir
}

func TestLaunchOpensATmuxWindowForTheAccount(t *testing.T) {
	e, f, dir := launchEnv(t)

	code := e.run("launch", "work", "--cwd", dir)

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	want := []runcmd.Call{{Name: "tmux", Args: []string{"new-window", "-P", "-c", dir, "/opt/clalt/bin/clalt", "run", "work"}}}
	if got := f.run.Calls(); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
	if out := e.out.String(); out != "opened work in tmux window main:3\n" {
		t.Errorf("out = %q", out)
	}
}

func TestLaunchOpensAKittyTab(t *testing.T) {
	e, f, dir := launchEnv(t)
	delete(f.env, "TMUX")
	f.env["KITTY_LISTEN_ON"] = "unix:/tmp/kitty"
	f.run.On("kitten", "@").Output("9\n")

	code := e.run("launch", "work", "--cwd="+dir, "--", "--model", "opus")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	want := []string{"@", "--to", "unix:/tmp/kitty", "launch", "--type=tab", "--cwd=" + dir,
		"/opt/clalt/bin/clalt", "run", "work", "--", "--model", "opus"}
	if got := f.run.Calls()[0].Args; !reflect.DeepEqual(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
	if out := e.out.String(); out != "opened work in kitty window 9\n" {
		t.Errorf("out = %q", out)
	}
}

func TestLaunchDefaultsToTheCurrentFolder(t *testing.T) {
	e, f, _ := launchEnv(t)
	here, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if code := e.run("launch", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := f.run.Calls()[0].Args[3]; got != here {
		t.Errorf("cwd = %q, want %q", got, here)
	}
}

func TestLaunchMakesARelativeFolderAbsolute(t *testing.T) {
	e, f, _ := launchEnv(t)
	here, _ := os.Getwd()

	if code := e.run("launch", "work", "--cwd", "."); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if got := f.run.Calls()[0].Args[3]; got != here {
		t.Errorf("cwd = %q, want %q", got, here)
	}
}

func TestLaunchWithoutATerminalPrintsTheCommand(t *testing.T) {
	e, f, dir := launchEnv(t)
	delete(f.env, "TMUX")

	code := e.run("launch", "work", "--cwd", dir, "--", "--resume", "a b")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	want := "no kitty socket or tmux here. run this in the terminal you want:\ncd " + terminal.ShellLine([]string{dir}) + " && clalt run work -- --resume 'a b'\n"
	if e.out.String() != want {
		t.Errorf("out = %q, want %q", e.out.String(), want)
	}
	if len(f.run.Calls()) != 0 {
		t.Errorf("calls = %v", f.run.Calls())
	}
}

func TestLaunchShowsAFailedCallAndExitsOne(t *testing.T) {
	e, f, dir := launchEnv(t)
	f.run = runcmd.NewFake()
	e.app.sdeps.runner = f.run
	f.run.On("tmux").Fail(1, "no server running\nsecond")

	code := e.run("launch", "work", "--cwd", dir)

	if code != 1 || !strings.Contains(e.errOut.String(), "no server running") || strings.Contains(e.errOut.String(), "second") {
		t.Errorf("exit %d, stderr %q", code, e.errOut.String())
	}
}
