package cli

import (
	"strings"
	"testing"
)

func TestParseDashboardArgs(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		live, demo bool
		err        string
	}{
		{"nothing", nil, false, false, ""},
		{"live", []string{"--live"}, true, false, ""},
		{"demo", []string{"--demo"}, false, true, ""},
		{"both is a mistake in either order", []string{"--demo", "--live"}, false, false, "--live has no effect with --demo"},
		{"live then demo", []string{"--live", "--demo"}, false, false, "--live has no effect with --demo"},
		{"flags after a name", []string{"work", "--demo"}, false, true, ""},
		{"an account name alone", []string{"work"}, false, false, ""},
		{"a repeated flag is fine", []string{"--live", "--live"}, true, false, ""},
		{"an unknown flag", []string{"--nope"}, false, false, "unknown flag --nope"},
		{"an unknown flag after a good one", []string{"--live", "--nope"}, false, false, "unknown flag --nope"},
		{"close to demo is not demo", []string{"--demos"}, false, false, "unknown flag --demos"},
		{"one dash is not demo", []string{"-demo"}, false, false, "unknown flag -demo"},
		{"demo without dashes is a name", []string{"demo"}, false, false, ""},
	}
	for _, c := range cases {
		live, demo, err := parseDashboardArgs(c.args)
		if live != c.live || demo != c.demo || (err == nil) != (c.err == "") || (err != nil && err.Error() != c.err) {
			t.Errorf("%s: parseDashboardArgs(%v) = live %v demo %v err %v, want live %v demo %v err %q", c.name, c.args, live, demo, err, c.live, c.demo, c.err)
		}
	}
}

func TestDashboardMistakesExitTwoWithAUsageLine(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"dashboard", "--nope"}, "error: unknown flag --nope"},
		{[]string{"dashboard", "--demo", "--live"}, "error: --live has no effect with --demo"},
	} {
		app, out, _ := newTestApp(t)
		if code := run(app, c.args); code != 2 {
			t.Errorf("%v exits %d, want 2", c.args, code)
		}
		got := out.String()
		if !strings.Contains(got, c.want) || !strings.Contains(got, "usage: clalt dashboard [--live | --demo]") {
			t.Errorf("%v prints the mistake and how to call it:\n%s", c.args, got)
		}
		if strings.Contains(got, "Accounts") {
			t.Errorf("%v must not start a dashboard:\n%s", c.args, got)
		}
	}
}

func TestDashboardDemoStillRunsAndExitsZero(t *testing.T) {
	app, out, _ := newTestApp(t)
	if code := run(app, []string{"dashboard", "--demo"}); code != 0 || !strings.Contains(out.String(), "Accounts") {
		t.Errorf("exit %d:\n%s", code, out.String())
	}
}

// without a terminal the demo prints one plain frame instead of starting the
// full screen program
func TestDashboardDemoWithoutATerminalPrintsOneFrame(t *testing.T) {
	app, out, _ := newTestApp(t)
	if err := app.cmdDashboardDemo(false); err != nil {
		t.Fatalf("cmdDashboardDemo: %v", err)
	}
	got := out.String()
	for _, want := range []string{"Accounts", "robin", "5 accounts, 3 running", "Signed in. Re-login due in 23 days.", "q quit"} {
		if !strings.Contains(got, want) {
			t.Errorf("the frame is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\x1b") {
		t.Error("a frame for a pipe has no escape codes")
	}
	for i := 0; i < len(got); i++ {
		if got[i] >= 0x80 {
			t.Fatal("a frame for a pipe is plain ascii")
		}
	}
	if lines := strings.Count(got, "\n"); lines != 30 {
		t.Errorf("30 frame lines, each ended by a newline, got %d newlines", lines)
	}
}

// the demo shows made up accounts only: one that is really on the machine is
// neither shown nor changed
func TestDashboardDemoNeverShowsOrTouchesARealAccount(t *testing.T) {
	app, out, dir := newTestApp(t)
	_ = app.Live.Write(liveCred("tok-real"))
	_ = app.cmdAdd([]string{"realaccount"})
	before, _ := app.Store.List()
	out.Reset()
	if err := app.cmdDashboardDemo(false); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); strings.Contains(got, "realaccount") || !strings.Contains(got, "robin") {
		t.Errorf("the demo lists its own accounts and not the real one:\n%s", got)
	}
	after, _ := app.Store.List()
	if len(after) != len(before) || len(after) != 1 || after[0].Name != "realaccount" {
		t.Errorf("the demo changed the account list in %s: %v then %v", dir, before, after)
	}
	if live, err := app.Live.Read(); err != nil || live.AccessToken != "tok-real" {
		t.Errorf("the live login is untouched: %v %v", live.AccessToken, err)
	}
}

func TestUsageMentionsTheDemoPreview(t *testing.T) {
	app, out, _ := newTestApp(t)
	app.usage()
	if !strings.Contains(out.String(), "clalt dashboard --demo") {
		t.Errorf("the help should list the demo flag:\n%s", out.String())
	}
}
