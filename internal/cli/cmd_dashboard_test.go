package cli

import (
	"strings"
	"testing"
)

func TestParseDashboardArgs(t *testing.T) {
	cases := []struct {
		name string
		args []string
		demo bool
		err  string
	}{
		{"nothing", nil, false, ""},
		{"demo", []string{"--demo"}, true, ""},
		{"flags after a name", []string{"work", "--demo"}, true, ""},
		{"an account name alone", []string{"work"}, false, ""},
		{"a repeated flag is fine", []string{"--demo", "--demo"}, true, ""},
		{"an unknown flag", []string{"--nope"}, false, "unknown flag --nope"},
		{"an unknown flag after a good one", []string{"--demo", "--nope"}, false, "unknown flag --nope"},
		{"close to demo is not demo", []string{"--demos"}, false, "unknown flag --demos"},
		{"one dash is not demo", []string{"-demo"}, false, "unknown flag -demo"},
		{"demo without dashes is a name", []string{"demo"}, false, ""},
	}
	for _, c := range cases {
		demo, err := parseDashboardArgs(c.args)
		if demo != c.demo || (err == nil) != (c.err == "") || (err != nil && err.Error() != c.err) {
			t.Errorf("%s: parseDashboardArgs(%v) = demo %v err %v, want demo %v err %q", c.name, c.args, demo, err, c.demo, c.err)
		}
	}
}

func TestDashboardMistakesExitTwoWithAUsageLine(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"dashboard", "--nope"}, "error: unknown flag --nope"},
	} {
		app, out, _ := newTestApp(t)
		if code := run(app, c.args); code != 2 {
			t.Errorf("%v exits %d, want 2", c.args, code)
		}
		got := out.String()
		if !strings.Contains(got, c.want) || !strings.Contains(got, "usage: clalt dashboard [--demo]") {
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
// neither shown nor changed, and claude is never asked about it
func TestDashboardDemoNeverShowsOrTouchesARealAccount(t *testing.T) {
	e := newEnv(t)
	e.signIn("realaccount", "r@example.com")
	before, _ := e.app.Store.List()
	asked := len(e.claude.calls("status"))

	if err := e.app.cmdDashboardDemo(false); err != nil {
		t.Fatal(err)
	}

	if got := e.out.String(); strings.Contains(got, "realaccount") || !strings.Contains(got, "robin") {
		t.Errorf("the demo lists its own accounts and not the real one:\n%s", got)
	}
	after, _ := e.app.Store.List()
	if len(after) != len(before) || len(after) != 1 || after[0].Name != "realaccount" || after[0].Meta != before[0].Meta {
		t.Errorf("the demo changed the account list: %v then %v", before, after)
	}
	if got := len(e.claude.calls("status")); got != asked {
		t.Errorf("claude was asked %d more times", got-asked)
	}
}

func TestUsageMentionsTheDemoPreview(t *testing.T) {
	app, out, _ := newTestApp(t)
	app.usage()
	if !strings.Contains(out.String(), "clalt dashboard --demo") {
		t.Errorf("the help should list the demo flag:\n%s", out.String())
	}
}
