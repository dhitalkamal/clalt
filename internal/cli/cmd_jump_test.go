package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/runcmd"
	"github.com/dhitalkamal/clalt/internal/terminal"
)

// jumpEnv is an account "work" and a few running sessions in fake deps.
func jumpEnv(t *testing.T) (*testEnv, *fakeDeps) {
	t.Helper()
	e := newEnv(t)
	e.signIn("work", "a@example.com")
	f := e.useFakeDeps()
	id := e.account("work").Name
	f.running = []terminal.Session{
		{Handle: "alpha", Account: id, ClaudeSessionID: "1234abcd-0000", Cwd: "/src/a",
			Location: terminal.Location{Kind: terminal.Kitty, KittyWindow: 7, KittyListenOn: "unix:/tmp/k"}},
		{Handle: "beta", Account: id, ClaudeSessionID: "9999ffff-0000", Cwd: "/src/b",
			Location: terminal.Location{Kind: terminal.Tmux, TmuxPane: "%5", TmuxSocket: "/tmp/tmux-1/default"}},
		{Handle: "gamma", Account: id, Cwd: "/src/c", Location: terminal.Location{Kind: terminal.IDE, IDE: "vscode"}},
		{Handle: "delta", Account: id, Cwd: "/src/d", Location: terminal.Location{Kind: terminal.Plain}},
	}
	return e, f
}

func TestJumpFocusesAKittyWindow(t *testing.T) {
	e, f := jumpEnv(t)

	code := e.run("jump", "alpha")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	want := []runcmd.Call{{Name: "kitten", Args: []string{"@", "--to", "unix:/tmp/k", "focus-window", "--match", "id:7"}}}
	if got := f.run.Calls(); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
	if e.out.String() != "brought alpha (work) to the front\n" {
		t.Errorf("out = %q", e.out.String())
	}
}

func TestJumpFocusesATmuxPane(t *testing.T) {
	e, f := jumpEnv(t)

	if code := e.run("jump", "beta"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	calls := f.run.Calls()
	if len(calls) != 3 || calls[2].Args[2] != "switch-client" {
		t.Errorf("calls = %v", calls)
	}
}

func TestJumpSaysWhenTmuxCouldNotSwitchTheClient(t *testing.T) {
	e, f := jumpEnv(t)
	f.run.On("tmux", "-S", "/tmp/tmux-1/default", "switch-client").Fail(1, "no current client")

	code := e.run("jump", "beta")

	if code != 0 || !strings.Contains(e.out.String(), "no current client") {
		t.Errorf("exit %d, out %q", code, e.out.String())
	}
}

func TestJumpMatchesAHandlePrefixAndASessionIdPrefix(t *testing.T) {
	for _, arg := range []string{"alph", "1234ab", "alpha"} {
		e, f := jumpEnv(t)

		code := e.run("jump", arg)

		if code != 0 || len(f.run.Calls()) != 1 {
			t.Errorf("%s: exit %d, calls %v, stderr %q", arg, code, f.run.Calls(), e.errOut.String())
		}
	}
}

func TestJumpPrefersAnExactHandleOverALongerOne(t *testing.T) {
	e, f := jumpEnv(t)
	f.running = append(f.running, terminal.Session{Handle: "alpha2", Cwd: "/x",
		Location: terminal.Location{Kind: terminal.Kitty, KittyWindow: 8, KittyListenOn: "unix:/tmp/k"}})

	code := e.run("jump", "alpha")

	if code != 0 || f.run.Calls()[0].Args[5] != "id:7" {
		t.Errorf("exit %d, calls %v", code, f.run.Calls())
	}
}

func TestJumpListsSeveralMatchesAndFocusesNone(t *testing.T) {
	e, f := jumpEnv(t)
	f.running = append(f.running, terminal.Session{Handle: "alpine", Account: e.account("work").Name, Cwd: "/src/\x1b[31mx"})

	code := e.run("jump", "al")

	if code != 1 {
		t.Errorf("exit %d", code)
	}
	for _, w := range []string{"alpha", "alpine", "work", "/src/a", "/src/"} {
		if !strings.Contains(e.errOut.String(), w) {
			t.Errorf("stderr %q lacks %q", e.errOut.String(), w)
		}
	}
	if strings.Contains(e.errOut.String(), "\x1b") || strings.Contains(e.errOut.String(), "beta") || len(f.run.Calls()) != 0 {
		t.Errorf("stderr %q, calls %v", e.errOut.String(), f.run.Calls())
	}
}

func TestJumpWithNoMatchSaysSo(t *testing.T) {
	e, f := jumpEnv(t)

	code := e.run("jump", "zzz")

	if code != 1 || !strings.Contains(e.errOut.String(), `no running session matches "zzz"`) || len(f.run.Calls()) != 0 {
		t.Errorf("exit %d, stderr %q", code, e.errOut.String())
	}
}

func TestJumpTakesOneArgument(t *testing.T) {
	for _, args := range [][]string{{"jump"}, {"jump", "a", "b"}, {"jump", "--x"}} {
		e, f := jumpEnv(t)

		code := e.run(args...)

		if code != 2 || !strings.Contains(e.errOut.String(), "usage: clalt jump <session>") || len(f.run.Calls()) != 0 {
			t.Errorf("%v: exit %d, stderr %q", args, code, e.errOut.String())
		}
	}
}

func TestJumpToAnIDEOrPlainTerminalPrintsWhereItRuns(t *testing.T) {
	cases := map[string][]string{
		"gamma": {"vscode", "/src/c", "work"},
		"delta": {"terminal", "/src/d", "work"},
	}
	for handle, want := range cases {
		e, f := jumpEnv(t)

		code := e.run("jump", handle)

		if code != 0 || len(f.run.Calls()) != 0 {
			t.Fatalf("%s: exit %d, calls %v, stderr %q", handle, code, f.run.Calls(), e.errOut.String())
		}
		for _, w := range want {
			if !strings.Contains(e.out.String(), w) {
				t.Errorf("%s: out %q lacks %q", handle, e.out.String(), w)
			}
		}
	}
}

func TestJumpShowsTheFirstStderrLineOfAFailedCall(t *testing.T) {
	e, f := jumpEnv(t)
	f.run.On("kitten").Fail(1, "no such window\nsecond line")

	code := e.run("jump", "alpha")

	if code != 1 || !strings.Contains(e.errOut.String(), "no such window") || strings.Contains(e.errOut.String(), "second line") {
		t.Errorf("exit %d, stderr %q", code, e.errOut.String())
	}
}

func TestJumpRefusesARecordWithABadPane(t *testing.T) {
	e, f := jumpEnv(t)
	f.running[1].Location.TmuxPane = "5;x"

	code := e.run("jump", "beta")

	if code != 1 || len(f.run.Calls()) != 0 {
		t.Errorf("exit %d, calls %v", code, f.run.Calls())
	}
}

func TestJumpShowsAFailureToReadTheSessions(t *testing.T) {
	e, f := jumpEnv(t)
	f.runErr = os.ErrPermission

	code := e.run("jump", "alpha")

	if code != 1 || len(f.run.Calls()) != 0 {
		t.Errorf("exit %d, calls %v", code, f.run.Calls())
	}
}

func TestJumpCreatesNoStateWhenItFailsEarly(t *testing.T) {
	e := newEnv(t)
	e.useFakeDeps()

	if code := e.run("jump", "nothing"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(e.home, ".config", "clalt")); err == nil {
		t.Error("jump created the clalt folder")
	}
}
