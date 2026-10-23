package notify

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dhitalkamal/clalt/internal/runcmd"
)

func send(t *testing.T, goos string, n Note) (*runcmd.Fake, error) {
	t.Helper()
	f := runcmd.NewFake()
	nf, ok := System(f, goos)
	if !ok {
		t.Fatalf("no notifier for %s", goos)
	}
	return f, nf.Notify(context.Background(), n)
}

func TestMacArgvKeepsTextOutOfTheScript(t *testing.T) {
	f, err := send(t, "darwin", Note{Title: "work needs you", Text: `say "hi" & quit`})
	if err != nil {
		t.Fatal(err)
	}
	want := []runcmd.Call{{Name: "osascript", Args: []string{
		"-e", "on run argv",
		"-e", "display notification (item 3 of argv) with title (item 1 of argv) subtitle (item 2 of argv)",
		"-e", "end run",
		"clalt", "work needs you", `say "hi" & quit`}}}
	if got := f.Calls(); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v\nwant %v", got, want)
	}
}

func TestMacLeadingDashGetsASpace(t *testing.T) {
	f, _ := send(t, "darwin", Note{Title: "-t", Text: "-e boom"})
	a := f.Calls()[0].Args
	if a[len(a)-2] != " -t" || a[len(a)-1] != " -e boom" {
		t.Errorf("args = %q", a)
	}
}

func TestLinuxArgvEscapesBody(t *testing.T) {
	f, err := send(t, "linux", Note{Title: "a", Text: `x\n <b> & y`})
	if err != nil {
		t.Fatal(err)
	}
	want := []runcmd.Call{{Name: "notify-send", Args: []string{"--app-name=clalt", "--", "a", `x\\n &lt;b&gt; &amp; y`}}}
	if got := f.Calls(); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v\nwant %v", got, want)
	}
}

func TestSystemPicksByPlatform(t *testing.T) {
	if n, ok := System(runcmd.NewFake().Without("notify-send"), "linux"); ok || n != nil {
		t.Error("linux without notify-send has a notifier")
	}
	if n, ok := System(runcmd.NewFake(), "windows"); ok || n != nil {
		t.Error("windows has a notifier")
	}
	if _, ok := System(runcmd.NewFake().Without("notify-send"), "darwin"); !ok {
		t.Error("darwin has no notifier")
	}
}

func TestTextIsCleaned(t *testing.T) {
	f, _ := send(t, "darwin", Note{Title: "t", Text: "line one\n\nline\ttwo\x00\x1b[31m  end\r"})
	a := f.Calls()[0].Args
	if got := a[len(a)-1]; got != "line one line two [31m end" {
		t.Errorf("text = %q", got)
	}
}

func TestTextIsCutOnARuneBoundary(t *testing.T) {
	f, _ := send(t, "darwin", Note{Title: "t", Text: strings.Repeat("\u00e9", 200)})
	a := f.Calls()[0].Args
	got := a[len(a)-1]
	if !utf8.ValidString(got) || utf8.RuneCountInString(got) != 120 {
		t.Errorf("text has %d runes, valid %v", utf8.RuneCountInString(got), utf8.ValidString(got))
	}
}

func TestEmptyTitleBecomesClalt(t *testing.T) {
	f, _ := send(t, "darwin", Note{Title: " \n ", Text: "x"})
	a := f.Calls()[0].Args
	if a[len(a)-2] != "clalt" {
		t.Errorf("title = %q", a[len(a)-2])
	}
}

func TestNotifyReturnsTheRunError(t *testing.T) {
	f := runcmd.NewFake()
	f.On("osascript").Fail(1, "boom")
	n, _ := System(f, "darwin")
	if err := n.Notify(context.Background(), Note{Title: "t", Text: "x"}); err == nil {
		t.Error("error swallowed")
	}
}

func TestTestSendsASample(t *testing.T) {
	f := runcmd.NewFake()
	n, _ := System(f, "darwin")
	if err := Test(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	a := f.Calls()[0].Args
	if !strings.Contains(a[len(a)-2]+a[len(a)-1], "clalt notifications work") {
		t.Errorf("args = %q", a)
	}
}
