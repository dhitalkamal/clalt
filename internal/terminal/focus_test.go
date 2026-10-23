package terminal

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/runcmd"
)

func TestFocusKittyRunsFocusWindow(t *testing.T) {
	f := runcmd.NewFake()
	d := Driver{Run: f}

	_, err := d.Focus(context.Background(), Location{Kind: Kitty, KittyWindow: 7, KittyListenOn: "unix:/tmp/k"})

	if err != nil {
		t.Fatal(err)
	}
	want := runcmd.Call{Name: "kitten", Args: []string{"@", "--to", "unix:/tmp/k", "focus-window", "--match", "id:7"}}
	if got := lastCall(t, f); !reflect.DeepEqual(got, want) {
		t.Errorf("call = %v, want %v", got, want)
	}
}

func TestFocusKittyRefusesBadInputWithoutACall(t *testing.T) {
	bad := []Location{
		{Kind: Kitty, KittyWindow: 0, KittyListenOn: "unix:/k"},
		{Kind: Kitty, KittyWindow: -3, KittyListenOn: "unix:/k"},
		{Kind: Kitty, KittyWindow: 1, KittyListenOn: ""},
		{Kind: Kitty, KittyWindow: 1, KittyListenOn: "/tmp/k"},
		{Kind: Kitty, KittyWindow: 1, KittyListenOn: "--exec"},
	}
	for _, loc := range bad {
		f := runcmd.NewFake()

		_, err := Driver{Run: f}.Focus(context.Background(), loc)

		if err == nil || len(f.Calls()) != 0 {
			t.Errorf("%+v: err = %v, calls = %v", loc, err, f.Calls())
		}
	}
}

func TestFocusKittyAcceptsTcp(t *testing.T) {
	f := runcmd.NewFake()

	_, err := Driver{Run: f}.Focus(context.Background(), Location{Kind: Kitty, KittyWindow: 2, KittyListenOn: "tcp:localhost:5000"})

	if err != nil || len(f.Calls()) != 1 {
		t.Errorf("err = %v, calls = %v", err, f.Calls())
	}
}

func TestFocusTmuxRunsThreeCalls(t *testing.T) {
	f := runcmd.NewFake()

	res, err := Driver{Run: f}.Focus(context.Background(), Location{Kind: Tmux, TmuxPane: "%5", TmuxSocket: "/tmp/tmux-1/default"})

	if err != nil || res.Note != "" {
		t.Fatalf("err = %v, note = %q", err, res.Note)
	}
	want := []runcmd.Call{
		{Name: "tmux", Args: []string{"-S", "/tmp/tmux-1/default", "select-window", "-t", "%5"}},
		{Name: "tmux", Args: []string{"-S", "/tmp/tmux-1/default", "select-pane", "-t", "%5"}},
		{Name: "tmux", Args: []string{"-S", "/tmp/tmux-1/default", "switch-client", "-t", "%5"}},
	}
	if got := f.Calls(); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

func TestFocusTmuxWithoutASocketLeavesOutTheFlag(t *testing.T) {
	f := runcmd.NewFake()

	Driver{Run: f}.Focus(context.Background(), Location{Kind: Tmux, TmuxPane: "%12"})

	if got := f.Calls()[0].Args; !reflect.DeepEqual(got, []string{"select-window", "-t", "%12"}) {
		t.Errorf("args = %v", got)
	}
}

func TestFocusTmuxRefusesABadPaneWithoutACall(t *testing.T) {
	for _, pane := range []string{"", "5", "%", "%a", "%5;x", "-t", "%5\n"} {
		f := runcmd.NewFake()

		_, err := Driver{Run: f}.Focus(context.Background(), Location{Kind: Tmux, TmuxPane: pane})

		if err == nil || len(f.Calls()) != 0 {
			t.Errorf("pane %q: err = %v, calls = %v", pane, err, f.Calls())
		}
	}
}

func TestFocusTmuxFailureOfTheFirstTwoIsAnError(t *testing.T) {
	for _, verb := range []string{"select-window", "select-pane"} {
		f := runcmd.NewFake()
		f.On("tmux", verb).Fail(1, "can't find pane: %5")

		_, err := Driver{Run: f}.Focus(context.Background(), Location{Kind: Tmux, TmuxPane: "%5"})

		var fail *runcmd.Failure
		if !errors.As(err, &fail) {
			t.Errorf("%s: err = %v", verb, err)
		}
	}
}

func TestFocusTmuxSwitchClientFailureIsOnlyANote(t *testing.T) {
	f := runcmd.NewFake()
	f.On("tmux", "switch-client").Fail(1, "no current client")

	res, err := Driver{Run: f}.Focus(context.Background(), Location{Kind: Tmux, TmuxPane: "%5"})

	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(res.Note, "no current client") {
		t.Errorf("note = %q", res.Note)
	}
}

func TestFocusIDEAndPlainCannotFocus(t *testing.T) {
	for _, loc := range []Location{{Kind: IDE, IDE: "vscode"}, {Kind: Plain}, {Kind: "weird"}} {
		f := runcmd.NewFake()

		_, err := Driver{Run: f}.Focus(context.Background(), loc)

		var cf *CannotFocus
		if !errors.As(err, &cf) || cf.Kind != loc.Kind || cf.IDE != loc.IDE || len(f.Calls()) != 0 {
			t.Errorf("%+v: err = %v, calls = %v", loc, err, f.Calls())
		}
	}
}

func TestFocusWithoutTheBinaryIsAClearError(t *testing.T) {
	for _, loc := range []Location{
		{Kind: Kitty, KittyWindow: 1, KittyListenOn: "unix:/k"},
		{Kind: Tmux, TmuxPane: "%1"},
	} {
		f := runcmd.NewFake().Without("kitten", "tmux")

		_, err := Driver{Run: f}.Focus(context.Background(), loc)

		if err == nil || !strings.Contains(err.Error(), "not on PATH") || len(f.Calls()) != 0 {
			t.Errorf("%+v: err = %v, calls = %v", loc, err, f.Calls())
		}
	}
}
