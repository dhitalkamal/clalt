package terminal

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/runcmd"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func lastCall(t *testing.T, f *runcmd.Fake) runcmd.Call {
	t.Helper()
	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("calls = %v, want exactly one", calls)
	}
	return calls[0]
}

func TestLaunchInTmuxRunsNewWindow(t *testing.T) {
	f := runcmd.NewFake()
	f.On("tmux", "new-window").Output("main:3\n")
	d := Driver{Run: f, Env: envOf(map[string]string{"TMUX": "/tmp/t,1,0", "PATH": "/usr/bin", "XDG_CONFIG_HOME": "/x"})}

	where, err := d.Launch(context.Background(), LaunchSpec{Exe: "/bin/clalt", Account: "work", Cwd: "/src", Args: []string{"--resume", "a b"}})

	if err != nil {
		t.Fatal(err)
	}
	want := runcmd.Call{Name: "tmux", Args: []string{"new-window", "-P", "-c", "/src",
		"env", "PATH=/usr/bin", "XDG_CONFIG_HOME=/x", "/bin/clalt", "run", "work", "--", "--resume", "a b"}}
	if got := lastCall(t, f); !reflect.DeepEqual(got, want) {
		t.Errorf("call = %v, want %v", got, want)
	}
	if where != "tmux window main:3" {
		t.Errorf("where = %q", where)
	}
}

func TestLaunchInKittyRunsLaunchTab(t *testing.T) {
	f := runcmd.NewFake()
	f.On("kitten").Output("42\n")
	d := Driver{Run: f, Env: envOf(map[string]string{"KITTY_LISTEN_ON": "unix:/tmp/k"})}

	where, err := d.Launch(context.Background(), LaunchSpec{Exe: "/bin/clalt", Account: "work", Cwd: "/src"})

	if err != nil {
		t.Fatal(err)
	}
	want := runcmd.Call{Name: "kitten", Args: []string{"@", "--to", "unix:/tmp/k", "launch", "--type=tab", "--cwd=/src",
		"/bin/clalt", "run", "work"}}
	if got := lastCall(t, f); !reflect.DeepEqual(got, want) {
		t.Errorf("call = %v, want %v", got, want)
	}
	if where != "kitty window 42" {
		t.Errorf("where = %q", where)
	}
}

func TestLaunchPrefersTmuxOverKitty(t *testing.T) {
	f := runcmd.NewFake()
	d := Driver{Run: f, Env: envOf(map[string]string{"TMUX": "/t,1,0", "KITTY_LISTEN_ON": "unix:/k"})}

	d.Launch(context.Background(), LaunchSpec{Exe: "/c", Account: "a", Cwd: "/"})

	if got := lastCall(t, f); got.Name != "tmux" {
		t.Errorf("ran %v", got)
	}
}

func TestLaunchFallsBackToKittyWhenTmuxIsMissing(t *testing.T) {
	f := runcmd.NewFake().Without("tmux")
	d := Driver{Run: f, Env: envOf(map[string]string{"TMUX": "/t,1,0", "KITTY_LISTEN_ON": "unix:/k"})}

	d.Launch(context.Background(), LaunchSpec{Exe: "/c", Account: "a", Cwd: "/"})

	if got := lastCall(t, f); got.Name != "kitten" {
		t.Errorf("ran %v", got)
	}
}

func TestLaunchWithNothingToUseIsErrNoTerminal(t *testing.T) {
	cases := map[string]struct {
		env  map[string]string
		miss []string
	}{
		"no variables":         {map[string]string{}, nil},
		"tmux var, no binary":  {map[string]string{"TMUX": "/t,1,0"}, []string{"tmux"}},
		"kitty var, no binary": {map[string]string{"KITTY_LISTEN_ON": "unix:/k"}, []string{"kitten"}},
	}
	for name, c := range cases {
		f := runcmd.NewFake().Without(c.miss...)
		d := Driver{Run: f, Env: envOf(c.env)}

		_, err := d.Launch(context.Background(), LaunchSpec{Exe: "/c", Account: "a", Cwd: "/"})

		if !errors.Is(err, ErrNoTerminal) || len(f.Calls()) != 0 {
			t.Errorf("%s: err = %v, calls = %v", name, err, f.Calls())
		}
	}
}

func TestLaunchPassesOnlyTheFourVariablesThatAreSet(t *testing.T) {
	f := runcmd.NewFake()
	d := Driver{Run: f, Env: envOf(map[string]string{
		"TMUX": "/t,1,0", "HOME": "/h", "SECRET": "s", "CLAUDE_CONFIG_DIR": "/c", "CLALT_CLAUDE_BIN": "/b/claude", "PATH": "",
	})}

	d.Launch(context.Background(), LaunchSpec{Exe: "/c/clalt", Account: "a", Cwd: "/w"})

	want := []string{"new-window", "-P", "-c", "/w", "env", "CLAUDE_CONFIG_DIR=/c", "CLALT_CLAUDE_BIN=/b/claude", "/c/clalt", "run", "a"}
	if got := lastCall(t, f).Args; !reflect.DeepEqual(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}

func TestLaunchOmitsEnvWhenNothingIsSet(t *testing.T) {
	f := runcmd.NewFake()
	d := Driver{Run: f, Env: envOf(map[string]string{"TMUX": "/t,1,0"})}

	d.Launch(context.Background(), LaunchSpec{Exe: "/c/clalt", Account: "a", Cwd: "/w"})

	want := []string{"new-window", "-P", "-c", "/w", "/c/clalt", "run", "a"}
	if got := lastCall(t, f).Args; !reflect.DeepEqual(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}

func TestLaunchReportsAFailedCall(t *testing.T) {
	f := runcmd.NewFake()
	f.On("tmux").Fail(1, "no server running\nmore")
	d := Driver{Run: f, Env: envOf(map[string]string{"TMUX": "/t,1,0"})}

	_, err := d.Launch(context.Background(), LaunchSpec{Exe: "/c", Account: "a", Cwd: "/"})

	var fail *runcmd.Failure
	if !errors.As(err, &fail) || !strings.Contains(err.Error(), "no server running") {
		t.Errorf("err = %v", err)
	}
}
