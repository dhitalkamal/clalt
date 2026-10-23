package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/term"

	"github.com/dhitalkamal/clalt/internal/config"
)

// a pipe has no terminal, so the dashboard prints one frame of the real accounts
func TestTheDashboardWithoutATerminalPrintsAFrameOfTheRealAccounts(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.signIn("home", "h@example.com")
	e.claude.signOut(e.folder("home"))

	if code := e.run("dashboard"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	out := e.out.String()
	for _, want := range []string{"Accounts", "work", "home", "signed out", "h@example.com"} {
		if !strings.Contains(out, want) {
			t.Errorf("the frame is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b") || strings.Count(out, "\n") != 40 {
		t.Errorf("a frame for a pipe is plain, 40 lines: %d newlines", strings.Count(out, "\n"))
	}
	if len(e.claude.calls("status")) == 0 {
		t.Error("the sign-in is asked from claude, not read from the record")
	}
}

func TestTheDashboardWithNoAccountsSaysHowToAddOne(t *testing.T) {
	e := newEnv(t)
	if code := e.run("dashboard"); code != 0 || !strings.Contains(e.out.String(), "clalt login <name>") {
		t.Errorf("exit %d: %s", code, e.out.String())
	}
}

func TestTheDashboardCleansWhatTheStateFileAndTranscriptsHold(t *testing.T) {
	e := hostileStateEnv(t)
	e.writeSwitchLog(`{"ts":"2026-08-23T00:00:00Z","account":"ok"}` + "\n")
	e.writeTranscriptOf(hostile(), `claude\u001b]0;PWNED\u0007\nerror: forged line`)

	if code := e.run("dashboard"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if out := e.out.String(); forgedIn(strings.TrimSuffix(out, "\n")) || strings.ContainsRune(out, '\u202e') || !strings.Contains(out, "lab") {
		t.Errorf("a frame with something forged in it:\n%q", out)
	}
}

// go test with stdout on a terminal started the dashboard and waited for keys
func TestTheDashboardAsksTheAppsStreamsAndNotTheProcessStdout(t *testing.T) {
	pty, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil || !term.IsTerminal(int(pty.Fd())) {
		t.Skip("no terminal device to stand in for stdout")
	}
	defer pty.Close()
	was := os.Stdout
	os.Stdout = pty
	defer func() { os.Stdout = was }()
	app, out, _ := newTestApp(t)

	done := make(chan int, 1)
	go func() { done <- run(app, []string{"dashboard", "--demo"}) }()

	select {
	case code := <-done:
		if code != 0 || !strings.Contains(out.String(), "Accounts") {
			t.Errorf("exit %d:\n%s", code, out.String())
		}
	case <-after(5):
		t.Fatal("it started the program on the terminal that stdout is")
	}
}

func TestInteractiveNeedsBothStreamsToBeTerminals(t *testing.T) {
	app, _, _ := newTestApp(t)
	if app.interactive() {
		t.Error("buffers are not terminals")
	}
}

func TestTheDashboardLogGetsCleanedOneLineEntriesAndIsPrivate(t *testing.T) {
	newEnv(t)
	path := filepath.Join(config.ClaltDir(), "logs", "dashboard.log")
	log := dashboardLogger(path)

	log("load failed: bad \x1b[31mred\nforged: line")
	log("second")

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 || hasControl(string(raw[:len(raw)-1])) || !strings.Contains(lines[0], "load failed: bad") || !strings.HasSuffix(lines[1], " second") {
		t.Errorf("log = %q", raw)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", fi.Mode().Perm())
	}
}

func TestTheDashboardLogIsRotatedWhenItGrowsBig(t *testing.T) {
	newEnv(t)
	path := filepath.Join(config.ClaltDir(), "logs", "dashboard.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), maxDashLog+1), 0o600); err != nil {
		t.Fatal(err)
	}

	dashboardLogger(path)("fresh")

	if raw, _ := os.ReadFile(path); !strings.HasSuffix(strings.TrimSpace(string(raw)), "fresh") || len(raw) > 200 {
		t.Errorf("the new log = %q", raw)
	}
	if _, err := os.Stat(path + ".old"); err != nil {
		t.Errorf("the big one is kept as .old: %v", err)
	}
}

func dashHost(e *testEnv) *dashboardHost { return &dashboardHost{app: e.app} }

func TestTheHostRunsTheCodeOfRunWithTheTerminalAndKeepsTheReasonOfARefusal(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	acc := e.account("work")

	l := dashHost(e).Run(acc)
	if err := l.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	argv, env, _ := e.claude.session()
	if len(argv) != 0 || env["CLAUDE_CONFIG_DIR"] != e.folder("work") {
		t.Errorf("session argv %v, folder %q", argv, env["CLAUDE_CONFIG_DIR"])
	}

	// signed out: the reason is printed, and the launch waits for enter
	e.claude.signOut(e.folder("work"))
	e.reset()
	e.answer("\n")
	err := dashHost(e).Run(acc).Run()
	if err == nil || !strings.Contains(e.errOut.String(), "run failed: work is not ready to run") || !strings.Contains(e.out.String(), "enter") {
		t.Errorf("err %v, stderr %q, stdout %q", err, e.errOut.String(), e.out.String())
	}
}

func TestTheHostTellsClaudesExitStatusApart(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.set("EXIT", "3")

	err := dashHost(e).Run(e.account("work")).Run()

	coder, ok := err.(interface{ ExitCode() int })
	if !ok || coder.ExitCode() != 3 {
		t.Errorf("err = %v", err)
	}
	if e.errOut.Len() != 0 && strings.Contains(e.errOut.String(), "failed") {
		t.Errorf("claude's own status is not a failure of clalt: %q", e.errOut.String())
	}
}

func TestTheHostSignsInThroughTheCodeOfLogin(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.signOut(e.folder("work"))
	e.claude.signInAs("w@example.com")

	if err := dashHost(e).Login(e.account("work")).Run(); err != nil {
		t.Fatal(err)
	}

	if !e.claude.signedIn(e.folder("work")) || e.account("work").Meta.SignedInAt.IsZero() {
		t.Error("the sign-in was made and recorded")
	}
}

func TestTheHostJudgesTheSignInLikeRunDoes(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	acc := e.account("work")

	in, err := dashHost(e).CheckSignIn(acc)
	if err != nil || !in.OK || in.Email != "w@example.com" || in.Plan != "max" {
		t.Fatalf("signed in: %+v, %v", in, err)
	}
	e.claude.plantLogin(e.folder("work"), "other@example.com")
	if in, _ := dashHost(e).CheckSignIn(acc); in.OK || !strings.Contains(in.Text, "different account") {
		t.Errorf("a different email: %+v", in)
	}
	e.claude.signOut(e.folder("work"))
	if in, _ := dashHost(e).CheckSignIn(acc); in.OK || in.Reason == 0 {
		t.Errorf("signed out: %+v", in)
	}
}

func TestTheHostSignsOutLikeLogout(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	if err := dashHost(e).SignOut(e.account("work")); err != nil {
		t.Fatal(err)
	}

	if e.claude.signedIn(e.folder("work")) || !e.account("work").Meta.SignedInAt.IsZero() {
		t.Error("the account is still signed in")
	}
	if e.out.Len() != 0 || e.errOut.Len() != 0 {
		t.Errorf("the dashboard owns the screen: %q %q", e.out.String(), e.errOut.String())
	}
}

func TestTheHostReadsTheVersionAndTheLinks(t *testing.T) {
	e := newEnv(t)
	e.seedShared()
	e.signIn("work", "w@example.com")
	h := dashHost(e)

	if v, err := h.ClaudeVersion(); err != nil || v != "2.1.303" {
		t.Errorf("version %q, %v", v, err)
	}
	e.claude.set("VERSION", "2.1.250")
	if v, err := h.ClaudeVersion(); err == nil || v != "2.1.250" || !strings.Contains(err.Error(), "older") {
		t.Errorf("an old claude: %q, %v", v, err)
	}
	if ok, err := h.LinksInSync(e.folder("work")); !ok || err != nil {
		t.Error("run synced the folder")
	}
	if err := os.Remove(filepath.Join(e.folder("work"), "settings.json")); err != nil {
		t.Fatal(err)
	}
	if ok, _ := h.LinksInSync(e.folder("work")); ok {
		t.Error("a missing link is out of sync")
	}
}

var _ = context.Background
